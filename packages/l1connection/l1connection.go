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
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/iotaledger/hive.go/log"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/iota.go/v4/builder"
	"github.com/iotaledger/iota.go/v4/nodeclient"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/util"
)

type Config struct {
	APIAddress    string
	INXAddress    string
	FaucetAddress string
}

type Client interface {
	// requests funds from faucet, waits for confirmation
	RequestFunds(kp cryptolib.VariantKeyPair, timeout ...time.Duration) error
	// creates a simple value transaction
	MakeSimpleValueTX(
		sender cryptolib.VariantKeyPair,
		recipientAddr iotago.Address,
		amount iotago.BaseToken,
		blockIssuerAccountID iotago.AccountID,
	) (*iotago.SignedTransaction, error)
	// sends a tx (build block, do tipselection, etc) and wait for confirmation
	PostTxAndWaitUntilConfirmation(tx *iotago.SignedTransaction, issuerID iotago.AccountID, signer cryptolib.VariantKeyPair, timeout ...time.Duration) (iotago.BlockID, error)
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

	bech32Addr := myAddress.Bech32(c.Bech32HRP())
	queries := []nodeclient.IndexerQuery{
		&api.BasicOutputsQuery{AddressBech32: bech32Addr},
		&api.FoundriesQuery{AccountAddressBech32: bech32Addr},
		&api.NFTsQuery{AddressBech32: bech32Addr},
		&api.AnchorsQuery{
			GovernorBech32: bech32Addr,
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

func (c *l1client) postBlockAndWaitUntilConfirmation(ctx context.Context, block *iotago.Block) (iotago.BlockID, error) {
	blockID, err := c.nodeAPIClient.SubmitBlock(ctx, block)
	if err != nil {
		return iotago.EmptyBlockID, fmt.Errorf("failed to submit block: %w", err)
	}

	c.log.LogInfof("Posted blockID %v", blockID.ToHex())

	return c.waitUntilBlockConfirmed(ctx, blockID)
}

func (c *l1client) PostTxAndWaitUntilConfirmation(tx *iotago.SignedTransaction, issuerID iotago.AccountID, signer cryptolib.VariantKeyPair, timeout ...time.Duration) (iotago.BlockID, error) {
	ctxWithTimeout, cancelContext := newCtx(c.ctx, timeout...)
	defer cancelContext()

	// build and post block
	block, err := c.blockFromTx(tx, issuerID, signer)
	if err != nil {
		return iotago.EmptyBlockID, err
	}
	blockID, err := c.postBlockAndWaitUntilConfirmation(ctxWithTimeout, block)
	if err != nil {
		return blockID, err
	}

	// get tx metadata
	txID, err := tx.Transaction.ID() // NOTE: must use the "unsigned tx ID" (it's different from just `tx.ID()`)
	if err != nil {
		return blockID, err
	}
	lo.Must0(util.WaitUntil(ctxWithTimeout, func() (util.WaitAction, error) {
		txMetadata, err := c.nodeAPIClient.TransactionMetadata(ctxWithTimeout, txID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return util.WaitActionKeepWaiting, nil
			}
			return util.WaitActionDone, err
		}
		if txMetadata.TransactionFailureReason != api.TxFailureNone {
			return util.WaitActionDone, fmt.Errorf("tx failed with reason: %v", txMetadata.TransactionFailureReason)
		}
		return util.WaitActionDone, nil
	},
		util.WaitOpts{TimeoutMsg: "failed to wait for tx metadata"},
	))
	return blockID, nil
}

// waitUntilBlockConfirmed waits until a given block is confirmed, it takes care of promotions/re-attachments for that block
func (c *l1client) waitUntilBlockConfirmed(ctx context.Context, blockID iotago.BlockID) (iotago.BlockID, error) {
	return blockID, util.WaitUntil(ctx, func() (util.WaitAction, error) {
		// poll the node for block confirmation state
		metadata, err := c.nodeAPIClient.BlockMetadataByBlockID(ctx, blockID)
		if err != nil {
			return util.WaitActionDone, fmt.Errorf("failed to get block metadata: %w", err)
		}
		// check if block was included
		switch metadata.BlockState {
		case api.BlockStateConfirmed, api.BlockStateFinalized:
			return util.WaitActionDone, nil // success

		case api.BlockStateRejected, api.BlockStateFailed:
			return util.WaitActionDone, fmt.Errorf("block was not included in the ledger.  LedgerInclusionState: %s, FailureReason: %d",
				metadata.BlockState, metadata.BlockFailureReason)
		case api.BlockStatePending, api.BlockStateAccepted:
			// do nothing, keep waiting
			return util.WaitActionKeepWaiting, nil
		default:
			panic(fmt.Errorf("uknown block state %s", metadata.BlockState))
		}
	},
		util.WaitOpts{TimeoutMsg: "failed to wait for block confimation"},
	)
}

func (c *l1client) GetAnchorOutput(anchorID iotago.AnchorID, timeout ...time.Duration) (iotago.OutputID, iotago.Output, error) {
	ctxWithTimeout, cancelContext := newCtx(c.ctx, timeout...)
	outputID, stateOutput, _, err := c.indexerClient.Anchor(ctxWithTimeout, anchorID.ToAddress().(*iotago.AnchorAddress))
	cancelContext()
	return *outputID, stateOutput, err
}

// RequestFunds implements L1Connection
// requests funds directly to the implicit account from a given pubkey
func (c *l1client) RequestFunds(kp cryptolib.VariantKeyPair, timeout ...time.Duration) error {
	implicitAccoutAddr := iotago.ImplicitAccountCreationAddressFromPubKey(kp.GetPublicKey().AsEd25519PubKey())
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
	var accountOutputs iotago.OutputSet

	lo.Must0(util.WaitUntil(ctxWithTimeout, func() (util.WaitAction, error) {
		accountOutputs, err = c.OutputMap(implicitAccoutAddr)
		if err != nil {
			return util.WaitActionDone, err
		}
		return len(accountOutputs) > len(initialAddrOutputs), nil
	},
		util.WaitOpts{TimeoutMsg: "faucet request timed-out while waiting for the issuerAccount to be present in the accounts ledger"},
	))

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

	// wait until  blockIssuerAccountID is available on the "accounts ledger"
	lo.Must0(
		util.WaitUntil(ctxWithTimeout, func() (util.WaitAction, error) {
			congestion, err2 := c.nodeAPIClient.Congestion(ctxWithTimeout, blockIssuerAccountID.ToAddress().(*iotago.AccountAddress), c.nodeAPIClient.LatestAPI().MaxBlockWork())
			if err2 != nil {
				if strings.Contains(err2.Error(), "account not found") {
					return util.WaitActionKeepWaiting, nil
				}
				return util.WaitActionDone, err2
			}
			if congestion.Ready {
				return util.WaitActionDone, nil
			}
			return util.WaitActionKeepWaiting, nil
		},
			util.WaitOpts{TimeoutMsg: "faucet request timed-out while waiting for funds to be available"},
		),
	)

	l1API := c.APIProvider().LatestAPI()
	txBuilder := builder.NewTransactionBuilder(l1API)
	txBuilder.AddInput(&builder.TxInput{
		UnlockTarget: kp.Address(),
		InputID:      outputToConvertID,
		Input:        outputToConvert,
	})

	issuerAccountOutput := &iotago.AccountOutput{
		Amount: 0,
		// Mana:           outputToConvert.StoredMana(),
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
	}
	// set the amount to the minimum possible SD
	issuerAccountOutput.Amount = lo.Must(l1API.StorageScoreStructure().MinDeposit(issuerAccountOutput))
	txBuilder.AddOutput(issuerAccountOutput)

	txBuilder.AddOutput(&iotago.BasicOutput{
		Amount: outputToConvert.BaseTokenAmount() - issuerAccountOutput.Amount,
		Mana:   0,
		UnlockConditions: []iotago.BasicOutputUnlockCondition{
			&iotago.AddressUnlockCondition{
				Address: kp.Address(),
			},
		},
	})

	blockIssuance, err := c.nodeAPIClient.BlockIssuance(c.ctx)
	if err != nil {
		return fmt.Errorf("failed to query block issuance info: %w", err)
	}

	latestCommitmentID, err := blockIssuance.LatestCommitment.ID()
	if err != nil {
		return fmt.Errorf("failed to get latest commitment ID: %w", err)
	}

	txBuilder.AddCommitmentInput(&iotago.CommitmentInput{CommitmentID: latestCommitmentID})
	txBuilder.AddBlockIssuanceCreditInput(&iotago.BlockIssuanceCreditInput{AccountID: blockIssuerAccountID})
	txBuilder.SetCreationSlot(blockIssuance.LatestCommitment.Slot)
	txBuilder.AllotMinRequiredManaAndStoreRemainingManaInOutput(
		txBuilder.CreationSlot(),
		blockIssuance.LatestCommitment.ReferenceManaCost,
		blockIssuerAccountID,
		1,
	)

	tx, err := txBuilder.Build(kp.AsAddressSigner())
	if err != nil {
		return err
	}

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
	sender cryptolib.VariantKeyPair,
	recipientAddr iotago.Address,
	amount iotago.BaseToken,
	blockIssuerAccountID iotago.AccountID,
) (*iotago.SignedTransaction, error) {
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
	storeManaOutputIndex := 0

	if inputSum > amount {
		txBuilder = txBuilder.AddOutput(&iotago.BasicOutput{
			Amount:           inputSum - amount,
			UnlockConditions: iotago.BasicOutputUnlockConditions{&iotago.AddressUnlockCondition{Address: recipientAddr}},
		})
		storeManaOutputIndex = 1
	}

	// mana ---
	blockIssuance, err := c.nodeAPIClient.BlockIssuance(c.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query block issuance info: %w", err)
	}
	txBuilder.AllotMinRequiredManaAndStoreRemainingManaInOutput(
		txBuilder.CreationSlot(),
		blockIssuance.LatestCommitment.ReferenceManaCost,
		blockIssuerAccountID,
		storeManaOutputIndex,
	)
	// ---

	return txBuilder.Build(
		sender.AsAddressSigner(),
	)
}

func (c *l1client) blockFromTx(signedTx *iotago.SignedTransaction, blockIssuerID iotago.AccountID, signer cryptolib.VariantKeyPair) (*iotago.Block, error) {
	bi, err := c.nodeAPIClient.BlockIssuance(c.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query block issuance info: %w", err)
	}

	// the issuing time of the blocks need to be monotonically increasing
	issuingTime := time.Now().UTC()
	if bi.LatestParentBlockIssuingTime.After(issuingTime) {
		issuingTime = bi.LatestParentBlockIssuingTime.Add(time.Nanosecond)
	}

	// Build a Block and post it.
	l1API := c.nodeAPIClient.CommittedAPI()
	return builder.NewBasicBlockBuilder(l1API).
		SlotCommitmentID(bi.LatestCommitment.MustID()).
		LatestFinalizedSlot(bi.LatestFinalizedSlot).
		StrongParents(bi.StrongParents).
		WeakParents(bi.WeakParents).
		ShallowLikeParents(bi.ShallowLikeParents).
		Payload(signedTx).
		CalculateAndSetMaxBurnedMana(bi.LatestCommitment.ReferenceManaCost).
		IssuingTime(issuingTime).
		SignWithSigner(blockIssuerID, signer.AsAddressSigner(), signer.Address()).
		Build()
}

// Health implements L1Client
func (c *l1client) Health(timeout ...time.Duration) (bool, error) {
	ctxWithTimeout, cancelContext := newCtx(context.Background(), timeout...)
	defer cancelContext()
	return c.nodeAPIClient.Health(ctxWithTimeout)
}

const defaultTimeout = 5 * time.Minute

func newCtx(ctx context.Context, timeout ...time.Duration) (context.Context, context.CancelFunc) {
	t := defaultTimeout
	if len(timeout) > 0 {
		t = timeout[0]
	}
	return context.WithTimeout(ctx, t)
}
