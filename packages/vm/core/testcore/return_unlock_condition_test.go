package testcore

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/transaction"
)

const (
	incCounterName     = "inccounter"
	incrementFn        = "increment"
	getCounterViewName = "getCounter"
	varCounter         = "counter"
)

func TestSendBack(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
	ch := env.NewChain()
	err := ch.DeployWasmContract(nil, incCounterName, "../../../../contracts/wasm/inccounter/pkg/inccounter_bg.wasm")
	require.NoError(t, err)

	err = ch.DepositBaseTokensToL2(10*isc.Million, nil)
	require.NoError(t, err)

	// send a normal request
	wallet, addr := env.NewKeyPairWithFunds()

	req := solo.NewCallParams(incCounterName, incrementFn).WithMaxAffordableGasBudget()
	_, _, err = ch.PostRequestSyncTx(req, wallet)
	require.NoError(t, err)

	// check counter increments
	ret, err := ch.CallView(incCounterName, getCounterViewName)
	require.NoError(t, err)
	counter, err := codec.DecodeInt64(ret.MustGet(varCounter))
	require.NoError(t, err)
	require.EqualValues(t, 1, counter)

	// send a custom request
	allOuts, allOutIDs := ch.Env.GetUnspentOutputs(addr)
	tx, err := transaction.NewRequestTransaction(transaction.NewRequestTransactionParams{
		SenderKeyPair:    wallet,
		SenderAddress:    addr,
		UnspentOutputs:   allOuts,
		UnspentOutputIDs: allOutIDs,
		Request: &isc.RequestParameters{
			TargetAddress:  ch.ChainID.AsAddress(),
			FungibleTokens: &isc.FungibleTokens{BaseTokens: 1 * isc.Million},
			Metadata: &isc.SendMetadata{
				TargetContract: isc.Hn(incCounterName),
				EntryPoint:     isc.Hn(incrementFn),
				GasBudget:      math.MaxUint64,
			},
		},
	})
	require.NoError(t, err)

	// tweak the tx before adding to the ledger, so the request output has a StorageDepositReturn unlock condition
	for i, out := range tx.Essence.Outputs {
		if out.FeatureSet().MetadataFeature() == nil {
			// skip if not the request output
			continue
		}
		customOut := out.Clone().(*iotago.BasicOutput)
		sendBackCondition := &iotago.StorageDepositReturnUnlockCondition{
			ReturnAddress: addr,
			Amount:        500,
		}
		customOut.Conditions = append(customOut.Conditions, sendBackCondition)
		tx.Essence.Outputs[i] = customOut
	}

	inputsCommitment := allOutIDs.OrderedSet(allOuts).MustCommitment()
	tx, err = transaction.CreateAndSignTx(allOutIDs, inputsCommitment, tx.Essence.Outputs, wallet, parameters.L1().Protocol.NetworkID())
	require.NoError(t, err)
	err = ch.Env.AddToLedger(tx)
	require.NoError(t, err)

	reqs, err := ch.Env.RequestsForChain(tx, ch.ChainID)
	require.NoError(ch.Env.T, err)
	results := ch.RunRequestsSync(reqs, "post")
	// TODO for now the request must be skipped, in the future this needs to be refactored, so that the request is handled as expected
	require.Len(t, results, 0)

	// check counter is still the same (1)
	ret, err = ch.CallView(incCounterName, getCounterViewName)
	require.NoError(t, err)
	counter, err = codec.DecodeInt64(ret.MustGet(varCounter))
	require.NoError(t, err)
	require.EqualValues(t, 1, counter)
}
