// to be used by utilities like: cluster-tool, wasp-cli, apilib, etc
package l1connection

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/iotaledger/hive.go/log"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/iota.go/v4/builder"
	"github.com/iotaledger/iota.go/v4/nodeclient"
	"github.com/iotaledger/wasp/packages/cryptolib"
)

const (
	pollConfirmedBlockInterval = 200 * time.Millisecond
)

type Config struct {
	APIAddress    string
	INXAddress    string
	FaucetAddress string
}

type Client interface {
	// requests funds from faucet, waits for confirmation
	RequestFunds(kp *cryptolib.KeyPair, timeout ...time.Duration) error
	// creates a simple value transaction
	MakeSimpleValueTX(
		sender *cryptolib.KeyPair,
		recipientAddr iotago.Address,
		amount iotago.BaseToken,
	) (*iotago.SignedTransaction, error)
	// sends a tx (build block, do tipselection, etc) and wait for confirmation
	PostTxAndWaitUntilConfirmation(tx *iotago.SignedTransaction, issuerID iotago.AccountID, signer *cryptolib.KeyPair, timeout ...time.Duration) (iotago.BlockID, error)
	// returns the outputs owned by a given address
	OutputMap(myAddress iotago.Address, timeout ...time.Duration) (iotago.OutputSet, error)
	// output
	GetAnchorOutput(anchorID iotago.AnchorID, timeout ...time.Duration) (iotago.OutputID, iotago.Output, error)
	// used to query the health endpoint of the node
	Health(timeout ...time.Duration) (bool, error)
	// APIProvider returns the L1 APIProvider
	APIProvider() iotago.APIProvider
	// Bech32HRP returns the bech32 humanly readable prefix for the current network
	Bech32HRP() iotago.NetworkPrefix
	// TokenInfo returns information about the L1 BaseToken
	TokenInfo(timeout ...time.Duration) (*api.InfoResBaseToken, error)
}

var _ Client = &l1client{}

type l1client struct {
	ctx           context.Context
	ctxCancel     context.CancelFunc
	indexerClient nodeclient.IndexerClient
	nodeAPIClient *nodeclient.Client
	log           log.Logger
	config        Config
}

func NewClient(config Config, log log.Logger, timeout ...time.Duration) Client {
	ctx, ctxCancel := context.WithCancel(context.Background())
	nodeAPIClient, err := nodeclient.New(config.APIAddress)
	if err != nil {
		panic(fmt.Errorf("error creating node connection: %w", err))
	}

	ctxWithTimeout, cancelContext := newCtx(ctx, timeout...)
	defer cancelContext()

	indexerClient, err := nodeAPIClient.Indexer(ctxWithTimeout)
	if err != nil {
		panic(fmt.Errorf("failed to get nodeclient indexer: %w", err))
	}

	return &l1client{
		ctx:           ctx,
		ctxCancel:     ctxCancel,
		indexerClient: indexerClient,
		nodeAPIClient: nodeAPIClient,
		log:           log.NewChildLogger("nc"),
		config:        config,
	}
}

// TokenInfo implements Client.
func (c *l1client) TokenInfo(timeout ...time.Duration) (*api.InfoResBaseToken, error) {
	ctxWithTimeout, cancelContext := newCtx(c.ctx, timeout...)
	defer cancelContext()

	res, err := c.nodeAPIClient.Info(ctxWithTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to query info: %w", err)
	}
	return res.BaseToken, nil
}

// OutputMap implements L1Connection
func (c *l1client) OutputMap(myAddress iotago.Address, timeout ...time.Duration) (iotago.OutputSet, error) {
	ctxWithTimeout, cancelContext := newCtx(c.ctx, timeout...)
	defer cancelContext()

	// TODO how to get the current epoch
	bech32Addr := myAddress.Bech32(c.Bech32HRP())
	queries := []nodeclient.IndexerQuery{
		&api.BasicOutputsQuery{AddressBech32: bech32Addr},
		&api.FoundriesQuery{AccountAddressBech32: bech32Addr},
		&api.NFTsQuery{AddressBech32: bech32Addr},
		&api.AnchorsQuery{
			GovernorBech32: bech32Addr,
			// IssuerBech32:                     "", // TODO needed? prob not
		},
		&api.AccountsQuery{AddressBech32: bech32Addr},
	}

	result := make(map[iotago.OutputID]iotago.Output)

	for _, query := range queries {
		res, err := c.indexerClient.Outputs(ctxWithTimeout, query)
		if err != nil {
			return nil, fmt.Errorf("failed to query address outputs: %w", err)
		}
		for res.Next() {
			outputs, err := res.Outputs(ctxWithTimeout)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch address outputs: %w", err)
			}

			outputIDs := res.Response.Items.MustOutputIDs()
			for i := range outputs {
				result[outputIDs[i]] = outputs[i]
			}
		}
	}
	return result, nil
}

func (c *l1client) postBlockAndWaitUntilConfirmation(block *iotago.Block, timeout ...time.Duration) (iotago.BlockID, error) {
	ctxWithTimeout, cancelContext := newCtx(c.ctx, timeout...)
	defer cancelContext()

	blockID, err := c.nodeAPIClient.SubmitBlock(ctxWithTimeout, block)
	if err != nil {
		return iotago.EmptyBlockID, fmt.Errorf("failed to submit block: %w", err)
	}

	c.log.LogInfof("Posted blockID %v", blockID.ToHex())

	return c.waitUntilBlockConfirmed(ctxWithTimeout, blockID)
}

func (c *l1client) PostTxAndWaitUntilConfirmation(tx *iotago.SignedTransaction, issuerID iotago.AccountID, signer *cryptolib.KeyPair, timeout ...time.Duration) (iotago.BlockID, error) {
	block, err := c.blockFromTx(tx, issuerID, signer)
	if err != nil {
		return iotago.EmptyBlockID, err
	}
	return c.postBlockAndWaitUntilConfirmation(block, timeout...)
}

// waitUntilBlockConfirmed waits until a given block is confirmed, it takes care of promotions/re-attachments for that block
func (c *l1client) waitUntilBlockConfirmed(ctx context.Context, blockID iotago.BlockID) (iotago.BlockID, error) {
	checkContext := func() error {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("failed to wait for block confimation within timeout: %w", err)
		}
		return nil
	}

	for {
		if err := checkContext(); err != nil {
			return iotago.EmptyBlockID, err
		}

		// poll the node for block confirmation state
		// TODO query by TXID instead (if the endpoint gets added)
		metadata, err := c.nodeAPIClient.BlockMetadataByBlockID(ctx, blockID)
		if err != nil {
			return iotago.EmptyBlockID, fmt.Errorf("failed to get block metadata: %w", err)
		}

		// TODO refactor to use inx.BlockMetadata_TRANSACTION_STATE_[...] variables (?)
		// check if block was included
		switch metadata.BlockState {
		case api.BlockStateConfirmed, api.BlockStateFinalized:
			return blockID, nil // success

		case api.BlockStateRejected, api.BlockStateFailed:
			return iotago.EmptyBlockID, fmt.Errorf("block was not included in the ledger.  LedgerInclusionState: %s, FailureReason: %d",
				metadata.BlockState, metadata.BlockFailureReason)

			// TODO: <lmoe> used to be  "pending", "conflicting".
			// Conflicting does not exist anymore, accepted seemed to be a sensible alternative here.
			// We need to revalidate the logic here soon anyway.
		case api.BlockStatePending, api.BlockStateAccepted:
			// do nothing

		default:
			panic(fmt.Errorf("uknown block state %s", metadata.BlockState))
		}

		time.Sleep(pollConfirmedBlockInterval)
	}
}

func (c *l1client) GetAnchorOutput(anchorID iotago.AnchorID, timeout ...time.Duration) (iotago.OutputID, iotago.Output, error) {
	ctxWithTimeout, cancelContext := newCtx(c.ctx, timeout...)
	outputID, stateOutput, _, err := c.indexerClient.Anchor(ctxWithTimeout, anchorID.ToAddress().(*iotago.AnchorAddress))
	cancelContext()
	return *outputID, stateOutput, err
}

// RequestFunds implements L1Connection
// requests funds directly to the implicit account from a given pubkey
func (c *l1client) RequestFunds(kp *cryptolib.KeyPair, timeout ...time.Duration) error {
	implicitAccoutAddr := iotago.ImplicitAccountCreationAddressFromPubKey(kp.GetPublicKey().AsEd25519PubKey())
	implicitAccoutAddrCasted := iotago.AccountAddress{}
	copy(implicitAccoutAddrCasted[:], implicitAccoutAddr[:])

	initialAddrOutputs, err := c.OutputMap(implicitAccoutAddr)
	if err != nil {
		return err
	}
	if len(initialAddrOutputs) != 0 {
		return fmt.Errorf("account already owns funds")
	}
	ctxWithTimeout, cancelContext := newCtx(c.ctx, timeout...)
	defer cancelContext()

	faucetReq := fmt.Sprintf("{\"address\":%q}", implicitAccoutAddr.Bech32(c.Bech32HRP()))
	faucetURL := fmt.Sprintf("%s/api/enqueue", c.config.FaucetAddress)
	httpReq, err := http.NewRequestWithContext(ctxWithTimeout, http.MethodPost, faucetURL, bytes.NewReader([]byte(faucetReq)))
	if err != nil {
		return fmt.Errorf("unable to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("unable to call faucet: %w", err)
	}
	if res.StatusCode != http.StatusAccepted {
		resBody, err2 := io.ReadAll(res.Body)
		defer res.Body.Close()
		if err2 != nil {
			return fmt.Errorf("faucet status=%v, unable to read response body: %w", res.Status, err2)
		}
		return fmt.Errorf("faucet call failed, response status=%v, body=%v", res.Status, string(resBody))
	}
	// wait until funds are available
	delay := 10 * time.Millisecond // in case the network is REALLY fast
	var accountOutputs iotago.OutputSet

	// Loop:
	// 	for {
	// 		select {
	// 		case <-ctxWithTimeout.Done():
	// 			return errors.New("faucet request timed-out while waiting for funds to be available")
	// 		case <-time.After(delay):
	// 			accountOutputs, err = c.OutputMap(implicitAccoutAddr)
	// 			if err != nil {
	// 				return err
	// 			}
	// 			if len(accountOutputs) > len(initialAddrOutputs) {
	// 				break Loop // success
	// 			}
	// 			delay = 1 * time.Second
	// 		}
	// 	}

	// wait until implicit account is available on the "accounts ledger"
Loop:
	for {
		select {
		case <-ctxWithTimeout.Done():
			return errors.New("faucet request timed-out while waiting for funds to be available")
		case <-time.After(delay):
			x, err2 := c.nodeAPIClient.Congestion(ctxWithTimeout, &implicitAccoutAddrCasted)
			if err2 != nil {
				return err2
			}
			println(x.Ready)
			if x.Ready {
				break Loop // success
			}
			delay = 1 * time.Second
		}
	}

	// convert the basic output into an account output + basicOutput
	if len(accountOutputs) != 1 {
		return errors.New("expected only 1 output to be owned after faucet request")
	}

	var outputToConvert iotago.Output
	var outputToConvertID iotago.OutputID
	for k, v := range accountOutputs {
		outputToConvertID = k
		outputToConvert = v
	}

	blockIssuerAccountID := iotago.AccountIDFromOutputID(outputToConvertID)

	txBuilder := builder.NewTransactionBuilder(c.APIProvider().LatestAPI())
	txBuilder.AddInput(&builder.TxInput{
		UnlockTarget: kp.Address(),
		InputID:      outputToConvertID,
		Input:        outputToConvert,
	})

	txBuilder.AddOutput(&iotago.AccountOutput{
		Amount:         100_000, // TODO calc minSD somehow?
		Mana:           outputToConvert.StoredMana(),
		AccountID:      blockIssuerAccountID,
		FoundryCounter: 0,
		UnlockConditions: []iotago.AccountOutputUnlockCondition{
			&iotago.AddressUnlockCondition{
				Address: kp.Address(),
			},
		},
		Features: []iotago.AccountOutputFeature{
			&iotago.BlockIssuerFeature{
				ExpirySlot: math.MaxUint32,
				BlockIssuerKeys: []iotago.BlockIssuerKey{
					iotago.Ed25519PublicKeyHashBlockIssuerKeyFromPublicKey(kp.GetPublicKey().AsHiveEd25519PubKey()),
				},
			},
		},
	})

	txBuilder.AddOutput(&iotago.BasicOutput{
		Amount: outputToConvert.BaseTokenAmount() - 100_000,
		Mana:   0,
		UnlockConditions: []iotago.BasicOutputUnlockCondition{
			&iotago.AddressUnlockCondition{
				Address: kp.Address(),
			},
		},
	})

	tx, err := txBuilder.Build(kp.AsAddressSigner())
	if err != nil {
		return err
	}

	// _, err = c.PostTxAndWaitUntilConfirmation(tx, iotago.AccountID(implicitAccoutAddr.ID()), kp)
	_, err = c.PostTxAndWaitUntilConfirmation(tx, blockIssuerAccountID, kp)
	return err
}

func (c *l1client) APIProvider() iotago.APIProvider {
	return c.nodeAPIClient
}

func (c *l1client) Bech32HRP() iotago.NetworkPrefix {
	return c.nodeAPIClient.LatestAPI().ProtocolParameters().Bech32HRP()
}

func (c *l1client) MakeSimpleValueTX(
	sender *cryptolib.KeyPair,
	recipientAddr iotago.Address,
	amount iotago.BaseToken,
) (*iotago.SignedTransaction, error) {
	senderAccountAddr := iotago.ImplicitAccountCreationAddressFromPubKey(sender.GetPublicKey().AsEd25519PubKey())

	senderAddr := sender.Address()
	senderAccountOutputs, err := c.OutputMap(senderAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get address outputs: %w", err)
	}
	txBuilder := builder.NewTransactionBuilder(c.APIProvider().LatestAPI())
	inputSum := iotago.BaseToken(0)
	for i, o := range senderAccountOutputs {
		if inputSum >= amount {
			break
		}
		oid := i
		out := o
		txBuilder = txBuilder.AddInput(&builder.TxInput{
			UnlockTarget: senderAddr,
			InputID:      oid,
			Input:        out,
		})
		inputSum += out.BaseTokenAmount()
	}
	if inputSum < amount {
		return nil, fmt.Errorf("not enough funds, have=%v, need=%v", inputSum, amount)
	}
	txBuilder = txBuilder.AddOutput(&iotago.BasicOutput{
		Amount:           amount,
		UnlockConditions: iotago.BasicOutputUnlockConditions{&iotago.AddressUnlockCondition{Address: recipientAddr}},
	})

	if inputSum > amount {
		txBuilder = txBuilder.AddOutput(&iotago.BasicOutput{
			Amount:           inputSum - amount,
			UnlockConditions: iotago.BasicOutputUnlockConditions{&iotago.AddressUnlockCondition{Address: recipientAddr}},
		})
	}

	// mana ---
	// TODO should probably use AllotMinRequiredManaAndStoreRemainingManaInOutput instead
	blockIssuance, err := c.nodeAPIClient.BlockIssuance(c.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query block issuance info: %w", err)
	}
	txBuilder.AllotAllMana(blockIssuance.LatestCommitment.Slot, iotago.AccountID(senderAccountAddr.ID()), blockIssuance.LatestCommitment.ReferenceManaCost)
	// txBuilder.AllotMinRequiredManaAndStoreRemainingManaInOutput(
	// 	blockIssuance.LatestCommitment.Slot,
	// 	blockIssuance.LatestCommitment.ReferenceManaCost,
	// 	iotago.AccountID(senderAccountAddr.ID()),
	// 	1, // ????????? not sure if correct - let's just try to use some output of this tx
	// )
	// ---

	return txBuilder.Build(
		sender.AsAddressSigner(),
	)
}

func (c *l1client) blockFromTx(tx *iotago.SignedTransaction, blockIssuerID iotago.AccountID, signer *cryptolib.KeyPair) (*iotago.Block, error) {
	bi, err := c.nodeAPIClient.BlockIssuance(c.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query block issuance info: %w", err)
	}

	// Build a Block and post it.
	l1API := c.nodeAPIClient.LatestAPI()
	return builder.NewBasicBlockBuilder(l1API).
		Payload(tx).
		StrongParents(bi.StrongParents).
		WeakParents(bi.WeakParents).
		SlotCommitmentID(bi.LatestCommitment.MustID()).
		ProtocolVersion(l1API.Version()).
		Sign(blockIssuerID, signer.GetPrivateKey().AsStdKey()).
		Build()
}

// Health implements L1Client
func (c *l1client) Health(timeout ...time.Duration) (bool, error) {
	ctxWithTimeout, cancelContext := newCtx(context.Background(), timeout...)
	defer cancelContext()
	return c.nodeAPIClient.Health(ctxWithTimeout)
}

const defaultTimeout = 1 * time.Minute

func newCtx(ctx context.Context, timeout ...time.Duration) (context.Context, context.CancelFunc) {
	t := defaultTimeout
	if len(timeout) > 0 {
		t = timeout[0]
	}
	return context.WithTimeout(ctx, t)
}
