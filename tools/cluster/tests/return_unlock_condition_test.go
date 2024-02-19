package tests

import (
	"testing"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
)

// builds a normal tx to post a request to inccounter, optionally adds SDRC
func buildTX(t *testing.T, env *ChainEnv, addr iotago.Address, keyPair *cryptolib.KeyPair, addSDRC bool) *iotago.SignedTransaction {
	panic("TODO rewrite")
	// outputs, err := env.Clu.L1Client().OutputMap(addr)
	// require.NoError(t, err)

	// tx, err := transaction.NewRequestTransaction(
	// 	keyPair,
	// 	addr,
	// 	outputs,
	// 	&isc.RequestParameters{
	// 		TargetAddress: env.Chain.ChainAddress(),
	// 		Assets:        isc.NewAssets(2*isc.Million, nil),
	// 		Metadata: &isc.SendMetadata{
	// 			Message:   inccounter.FuncIncCounter.Message(nil),
	// 			GasBudget: math.MaxUint64,
	// 		},
	// 	},
	// 	nil,
	// 	env.Clu.L1Client().APIProvider().LatestAPI().TimeProvider().SlotFromTime(time.Now()),
	// 	false,
	// 	env.Clu.L1Client().APIProvider(),
	// )
	// require.NoError(t, err)

	// if !addSDRC {
	// 	return tx
	// }

	// // tweak the tx , so the request output has a StorageDepositReturn unlock condition
	// for i, out := range tx.Transaction.Outputs {
	// 	if out.FeatureSet().Metadata() == nil {
	// 		// skip if not the request output
	// 		continue
	// 	}
	// 	customOut := out.Clone().(*iotago.BasicOutput)
	// 	sendBackCondition := &iotago.StorageDepositReturnUnlockCondition{
	// 		ReturnAddress: addr,
	// 		Amount:        1 * isc.Million,
	// 	}
	// 	customOut.UnlockConditions = append(customOut.UnlockConditions, sendBackCondition)
	// 	tx.Transaction.Outputs[i] = customOut
	// }

	// // parameters.L1API().TimeProvider().SlotFromTime(vmctx.task.Timestamp)
	// tx, err = transaction.CreateAndSignTx(
	// 	keyPair,
	// 	tx.Transaction.TransactionEssence.Inputs,
	// 	tx.Transaction.Outputs,
	// 	env.Clu.L1Client().APIProvider().LatestAPI().TimeProvider().SlotFromTime(time.Now()),
	// 	env.Clu.L1Client().APIProvider().LatestAPI(),
	// )
	// require.NoError(t, err)
	// return tx
}

// executed in cluster_test.go
func testSDRUC(t *testing.T, env *ChainEnv) {
	panic("TODO rewrite")
	// env.deployNativeIncCounterSC(0)
	// keyPair, addr, err := env.Clu.NewKeyPairWithFunds()
	// require.NoError(t, err)

	// initialBlockIdx, err := env.Chain.BlockIndex()
	// require.NoError(t, err)

	// // // send a request with Storage Deposit Return Unlock
	// txSDRC := buildTX(t, env, addr, keyPair, true)
	// _, err = env.Clu.L1Client().PostTxAndWaitUntilConfirmation(txSDRC)
	// require.NoError(t, err)

	// // wait some time and assert that the chain has not processed the request
	// time.Sleep(10 * time.Second) // don't like the sleep here, but not sure there is a better way to do this

	// // make sure the request is not picked up and the chain does not process it
	// currentBlockIndex, err := env.Chain.BlockIndex()
	// require.NoError(t, err)
	// require.EqualValues(t, initialBlockIdx, currentBlockIndex)

	// require.EqualValues(t, 0, env.getNativeContractCounter())

	// // send an equivalent request without StorageDepositReturnUnlockCondition
	// txNormal := buildTX(t, env, addr, keyPair, false)
	// _, err = env.Clu.L1Client().PostTxAndWaitUntilConfirmation(txNormal)
	// require.NoError(t, err)

	// _, err = env.Clu.MultiClient().WaitUntilAllRequestsProcessedSuccessfully(env.Chain.ChainID, txNormal, false, 1*time.Minute)
	// require.NoError(t, err)

	// require.EqualValues(t, 1, env.getNativeContractCounter())

	// currentBlockIndex2, err := env.Chain.BlockIndex()
	// require.NoError(t, err)
	// require.EqualValues(t, initialBlockIdx+1, currentBlockIndex2)
}
