package chainclient

import (
	"context"
	"math"
	"time"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/hexutil"
	"github.com/iotaledger/wasp/clients/apiclient"
	"github.com/iotaledger/wasp/clients/apiextensions"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/l1connection"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

// Client allows to interact with a specific chain in the node, for example to send on-ledger or off-ledger requests
type Client struct {
	Layer1Client l1connection.Client
	WaspClient   *apiclient.APIClient
	ChainID      isc.ChainID
	KeyPair      cryptolib.VariantKeyPair
}

// New creates a new chainclient.Client
func New(
	layer1Client l1connection.Client,
	waspClient *apiclient.APIClient,
	chainID isc.ChainID,
	keyPair cryptolib.VariantKeyPair,
) *Client {
	return &Client{
		Layer1Client: layer1Client,
		WaspClient:   waspClient,
		ChainID:      chainID,
		KeyPair:      keyPair,
	}
}

type PostRequestParams struct {
	Transfer                 *isc.Assets
	Nonce                    uint64
	NFT                      *isc.NFT
	Allowance                *isc.Assets
	gasBudget                gas.GasUnits
	AutoAdjustStorageDeposit bool
}

func (par *PostRequestParams) GasBudget() gas.GasUnits {
	if par.gasBudget == 0 {
		return math.MaxUint64
	}
	return par.gasBudget
}

func defaultParams(params ...PostRequestParams) PostRequestParams {
	if len(params) > 0 {
		return params[0]
	}
	return PostRequestParams{}
}

// PostRequest sends an on-ledger transaction with one request on it to the chain
func (c *Client) PostRequest(
	msg isc.Message,
	params ...PostRequestParams,
) (*iotago.SignedTransaction, error) {
	outputsSet, err := c.Layer1Client.OutputMap(c.KeyPair.Address())
	if err != nil {
		return nil, err
	}
	return c.post1RequestWithOutputs(msg, outputsSet, params...)
}

// PostNRequest sends n consecutive on-ledger transactions with one request on each, to the chain
func (c *Client) PostNRequests(
	msg isc.Message,
	requestsCount int,
	params ...PostRequestParams,
) ([]*iotago.SignedTransaction, error) {
	var err error
	outputs, err := c.Layer1Client.OutputMap(c.KeyPair.Address())
	if err != nil {
		return nil, err
	}
	transactions := make([]*iotago.SignedTransaction, requestsCount)
	for i := 0; i < requestsCount; i++ {
		transactions[i], err = c.post1RequestWithOutputs(msg, outputs, params...)
		if err != nil {
			return nil, err
		}
		txID, err := transactions[i].Transaction.ID()
		if err != nil {
			return nil, err
		}
		for _, input := range lo.Must(transactions[i].Transaction.Inputs()) {
			delete(outputs, input.OutputID())
		}
		for index, output := range transactions[i].Transaction.Outputs {
			if basicOutput, ok := output.(*iotago.BasicOutput); ok {
				if basicOutput.Ident().Equal(c.KeyPair.Address()) {
					outputID := iotago.OutputIDFromTransactionIDAndIndex(txID, uint16(index))
					outputs[outputID] = transactions[i].Transaction.Outputs[index]
				}
			}
		}
	}
	return transactions, nil
}

func (c *Client) post1RequestWithOutputs(
	msg isc.Message,
	outputs iotago.OutputSet,
	params ...PostRequestParams,
) (*iotago.SignedTransaction, error) {
	par := defaultParams(params...)
	tx, err := transaction.NewRequestTransaction(
		c.KeyPair,
		c.KeyPair.Address(),
		outputs,
		&isc.RequestParameters{
			TargetAddress:                 c.ChainID.AsAddress(),
			Assets:                        par.Transfer,
			AdjustToMinimumStorageDeposit: par.AutoAdjustStorageDeposit,
			Metadata: &isc.SendMetadata{
				Message:   msg,
				Allowance: par.Allowance,
				GasBudget: par.GasBudget(),
			},
		},
		par.NFT,
		c.Layer1Client.APIProvider().LatestAPI().TimeProvider().SlotFromTime(time.Now()),
		false,
		c.Layer1Client.APIProvider(),
	)
	if err != nil {
		return nil, err
	}

	blockIssuerID, err := util.BlockIssuerFromOutputs(outputs)
	if err != nil {
		return nil, err
	}

	_, err = c.Layer1Client.PostTxAndWaitUntilConfirmation(tx, blockIssuerID, c.KeyPair)
	return tx, err
}

func (c *Client) ISCNonce(ctx context.Context) (uint64, error) {
	result, _, err := c.WaspClient.ChainsApi.CallView(ctx, c.ChainID.Bech32(c.Layer1Client.Bech32HRP())).
		ContractCallViewRequest(apiclient.ContractCallViewRequest{
			ContractHName: accounts.Contract.Hname().String(),
			FunctionHName: accounts.ViewGetAccountNonce.Hname().String(),
			Arguments: apiextensions.JSONDictToAPIJSONDict(dict.Dict{
				accounts.ParamAgentID: isc.NewAgentID(c.KeyPair.Address()).Bytes(),
			}.JSONDict()),
		}).Execute()
	if err != nil {
		return 0, err
	}
	resultDict, err := apiextensions.APIJsonDictToDict(*result)
	if err != nil {
		return 0, err
	}
	return codec.Uint64.Decode(resultDict.Get(accounts.ParamAccountNonce))
}

// PostOffLedgerRequest sends an off-ledger tx via the wasp node web api
func (c *Client) PostOffLedgerRequest(
	ctx context.Context,
	msg isc.Message,
	params ...PostRequestParams,
) (isc.OffLedgerRequest, error) {
	par := defaultParams(params...)
	if par.Nonce == 0 {
		nonce, err := c.ISCNonce(ctx)
		if err != nil {
			return nil, err
		}
		par.Nonce = nonce
	}
	req := isc.NewOffLedgerRequest(c.ChainID, msg, par.Nonce, par.GasBudget())
	req.WithAllowance(par.Allowance)
	req.WithNonce(par.Nonce)
	signed := req.Sign(c.KeyPair)

	request := hexutil.EncodeHex(signed.Bytes())

	offLedgerRequest := apiclient.OffLedgerRequest{
		ChainId: c.ChainID.Bech32(c.Layer1Client.Bech32HRP()),
		Request: request,
	}
	_, err := c.WaspClient.RequestsApi.
		OffLedger(ctx).
		OffLedgerRequest(offLedgerRequest).
		Execute()

	return signed, err
}

func (c *Client) DepositFunds(n iotago.BaseToken) (*iotago.SignedTransaction, error) {
	return c.PostRequest(accounts.FuncDeposit.Message(), PostRequestParams{
		Transfer: isc.NewAssets(n, nil),
	})
}

func NewPostRequestParams() *PostRequestParams {
	return &PostRequestParams{
		Transfer:  isc.NewEmptyAssets(),
		Allowance: isc.NewEmptyAssets(),
	}
}

func (par *PostRequestParams) WithTransfer(transfer *isc.Assets) *PostRequestParams {
	par.Transfer = transfer
	return par
}

func (par *PostRequestParams) WithBaseTokens(i iotago.BaseToken) *PostRequestParams {
	par.Transfer.AddBaseTokens(i)
	return par
}

func (par *PostRequestParams) WithGasBudget(budget gas.GasUnits) *PostRequestParams {
	par.gasBudget = budget
	return par
}
