package tests

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/transaction"
)

// buils a normal tx to post a request to inccounter, optionally adds SDRC
func buildTX(t *testing.T, e *chainEnv, addr iotago.Address, keyPair *cryptolib.KeyPair, addSDRC bool) *iotago.Transaction {
	outputs, err := e.Clu.L1Client().OutputMap(addr)
	require.NoError(t, err)

	outputIDs := make(iotago.OutputIDs, len(outputs))
	i := 0
	for id := range outputs {
		outputIDs[i] = id
		i++
	}

	tx, err := transaction.NewRequestTransaction(transaction.NewRequestTransactionParams{
		SenderKeyPair:    keyPair,
		SenderAddress:    addr,
		UnspentOutputs:   outputs,
		UnspentOutputIDs: outputIDs,
		Request: &isc.RequestParameters{
			TargetAddress:  e.Chain.ChainAddress(),
			FungibleTokens: &isc.FungibleTokens{BaseTokens: 1 * isc.Million},
			Metadata: &isc.SendMetadata{
				TargetContract: incHname,
				EntryPoint:     incrementFuncHn,
				GasBudget:      math.MaxUint64,
			},
		},
	})
	require.NoError(t, err)

	if !addSDRC {
		return tx
	}

	// tweak the tx , so the request output has a StorageDepositReturn unlock condition
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

	inputsCommitment := outputIDs.OrderedSet(outputs).MustCommitment()
	tx, err = transaction.CreateAndSignTx(outputIDs, inputsCommitment, tx.Essence.Outputs, keyPair, parameters.L1().Protocol.NetworkID())
	require.NoError(t, err)
	return tx
}

// executed in cluster_test.go
func testSDRUC(t *testing.T, e *chainEnv) {
	e.deployWasmInccounter(0)
	keyPair, addr, err := e.Clu.NewKeyPairWithFunds()
	require.NoError(t, err)

	initialBlockIdx, err := e.Chain.BlockIndex()
	require.NoError(t, err)

	// // send a request with Storage Deposit Return Unlock
	txSDRC := buildTX(t, e, addr, keyPair, true)
	_, err = e.Clu.L1Client().PostTxAndWaitUntilConfirmation(txSDRC)
	require.NoError(t, err)

	// wait some time and assert that the chain has not processed the request
	time.Sleep(10 * time.Second) // don't like the sleep here, but not sure there is a better way to do this

	// make sure the request is not picked up and the chain does not process it
	currentBlockIndex, err := e.Chain.BlockIndex()
	require.NoError(t, err)
	require.EqualValues(t, initialBlockIdx, currentBlockIndex)

	e.expectCounter(0)

	// send an equivalent request without StorageDepositReturnUnlockCondition
	txNormal := buildTX(t, e, addr, keyPair, false)
	_, err = e.Clu.L1Client().PostTxAndWaitUntilConfirmation(txNormal)
	require.NoError(t, err)

	_, err = e.Clu.MultiClient().WaitUntilAllRequestsProcessedSuccessfully(e.Chain.ChainID, txNormal, 1*time.Minute)
	require.NoError(t, err)

	e.expectCounter(1)

	currentBlockIndex2, err := e.Chain.BlockIndex()
	require.NoError(t, err)
	require.EqualValues(t, initialBlockIdx+1, currentBlockIndex2)
}
