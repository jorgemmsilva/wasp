// to be used by utilities like: cluster-tool, wasp-cli, apilib, etc
package l1connection

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/iotaledger/hive.go/log"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/iota.go/v4/builder"
	"github.com/iotaledger/iota.go/v4/nodeclient"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
)

const (
	pollConfirmedBlockInterval = 200 * time.Millisecond
	promoteBlockCooldown       = 5 * time.Second

	// fundsFromFaucetAmount is how many base tokens are returned from the faucet.
	fundsFromFaucetAmount = 1000 * isc.Million
)

type Config struct {
	APIAddress    string
	INXAddress    string
	FaucetAddress string
}

type Client interface {
	// requests funds from faucet, waits for confirmation
	RequestFunds(addr iotago.Address, timeout ...time.Duration) error
	// sends a tx (including tipselection and local PoW if necessary) and waits for confirmation
	PostTxAndWaitUntilConfirmation(tx *iotago.SignedTransaction, timeout ...time.Duration) (iotago.BlockID, error)
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

// postBlock sends a block (including tipselection and local PoW if necessary).
func (c *l1client) postBlock(ctx context.Context, block *iotago.Block) (iotago.BlockID, error) {
	blockID, err := c.nodeAPIClient.SubmitBlock(ctx, block)
	if err != nil {
		return iotago.EmptyBlockID, fmt.Errorf("failed to submit block: %w", err)
	}

	c.log.LogInfof("Posted blockID %v", blockID.ToHex())

	return blockID, nil
}

// PostTx sends a tx (including tipselection and local PoW if necessary).
func (c *l1client) postTx(ctx context.Context, tx *iotago.SignedTransaction) (iotago.BlockID, error) {
	// Build a Block and post it.
	block, err := builder.NewBasicBlockBuilder(c.nodeAPIClient.LatestAPI()).Payload(tx).Build()
	if err != nil {
		return iotago.EmptyBlockID, fmt.Errorf("failed to build block: %w", err)
	}

	blockID, err := c.postBlock(ctx, block)
	if err != nil {
		return iotago.EmptyBlockID, err
	}

	txID, err := tx.ID()
	if err != nil {
		return iotago.EmptyBlockID, err
	}
	c.log.LogInfof("Posted transaction id %v", txID.ToHex())

	return blockID, nil
}

// PostTxAndWaitUntilConfirmation sends a tx (including tipselection and local PoW if necessary) and waits for confirmation.
func (c *l1client) PostTxAndWaitUntilConfirmation(tx *iotago.SignedTransaction, timeout ...time.Duration) (iotago.BlockID, error) {
	ctxWithTimeout, cancelContext := newCtx(c.ctx, timeout...)
	defer cancelContext()

	blockID, err := c.postTx(ctxWithTimeout, tx)
	if err != nil {
		return iotago.EmptyBlockID, err
	}

	return c.waitUntilBlockConfirmed(ctxWithTimeout, blockID, tx)
}

// waitUntilBlockConfirmed waits until a given block is confirmed, it takes care of promotions/re-attachments for that block
//
//nolint:gocyclo,funlen
func (c *l1client) waitUntilBlockConfirmed(ctx context.Context, blockID iotago.BlockID, payload iotago.Payload) (iotago.BlockID, error) {
	_, isTransactionPayload := payload.(*iotago.SignedTransaction)

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

		case api.BlockStateRejected:
			return iotago.EmptyBlockID, fmt.Errorf("block was not included in the ledger. IsTransaction: %t, LedgerInclusionState: %s, ConflictReason: %d",
				isTransactionPayload, metadata.BlockState, metadata.BlockFailureReason)

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
func (c *l1client) RequestFunds(addr iotago.Address, timeout ...time.Duration) error {
	initialAddrOutputs, err := c.OutputMap(addr)
	if err != nil {
		return err
	}
	ctxWithTimeout, cancelContext := newCtx(c.ctx, timeout...)
	defer cancelContext()

	faucetReq := fmt.Sprintf("{\"address\":%q}", addr.Bech32(c.Bech32HRP()))
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
		resBody, err := io.ReadAll(res.Body)
		defer res.Body.Close()
		if err != nil {
			return fmt.Errorf("faucet status=%v, unable to read response body: %w", res.Status, err)
		}
		return fmt.Errorf("faucet call failed, response status=%v, body=%v", res.Status, string(resBody))
	}
	// wait until funds are available
	delay := 10 * time.Millisecond // in case the network is REALLY fast
	for {
		select {
		case <-ctxWithTimeout.Done():
			return errors.New("faucet request timed-out while waiting for funds to be available")
		case <-time.After(delay):
			newOutputs, err := c.OutputMap(addr)
			if err != nil {
				return err
			}
			if len(newOutputs) > len(initialAddrOutputs) {
				return nil // success
			}
			delay = 1 * time.Second
		}
	}
}

// PostSimpleValueTX submits a simple value transfer TX.
// Can be used instead of the faucet API if the genesis key is known.
func (c *l1client) PostSimpleValueTX(
	sender *cryptolib.KeyPair,
	recipientAddr iotago.Address,
	amount iotago.BaseToken,
) error {
	tx, err := MakeSimpleValueTX(c, sender, recipientAddr, amount)
	if err != nil {
		return fmt.Errorf("failed to build a tx: %w", err)
	}

	_, err = c.PostTxAndWaitUntilConfirmation(tx)
	return err
}

func (c *l1client) APIProvider() iotago.APIProvider {
	return c.nodeAPIClient
}

func (c *l1client) Bech32HRP() iotago.NetworkPrefix {
	return c.nodeAPIClient.LatestAPI().ProtocolParameters().Bech32HRP()
}

func MakeSimpleValueTX(
	client Client,
	sender *cryptolib.KeyPair,
	recipientAddr iotago.Address,
	amount iotago.BaseToken,
) (*iotago.SignedTransaction, error) {
	senderAddr := sender.GetPublicKey().AsEd25519Address()
	senderOuts, err := client.OutputMap(senderAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get address outputs: %w", err)
	}
	txBuilder := builder.NewTransactionBuilder(client.APIProvider().LatestAPI())
	inputSum := iotago.BaseToken(0)
	for i, o := range senderOuts {
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
			UnlockConditions: iotago.BasicOutputUnlockConditions{&iotago.AddressUnlockCondition{Address: senderAddr}},
		})
	}
	tx, err := txBuilder.Build(
		sender.AsAddressSigner(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build a tx: %w", err)
	}
	return tx, nil
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
