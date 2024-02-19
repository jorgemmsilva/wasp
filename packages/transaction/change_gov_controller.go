package transaction

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
)

func NewChangeGovControllerTx(
	chainID iotago.AnchorID,
	newGovController iotago.Address,
	utxos iotago.OutputSet,
	creationSlot iotago.SlotIndex,
	l1API iotago.API,
	wallet cryptolib.VariantKeyPair,
) (*iotago.SignedTransaction, error) {
	panic("TODO fixme")
	// // find the correct chain UTXO
	// var chainOutput *iotago.AnchorOutput
	// var chainOutputID iotago.OutputID
	// for id, o := range utxos {
	// 	ao, ok := o.(*iotago.AnchorOutput)
	// 	if !ok {
	// 		continue
	// 	}
	// 	if util.AnchorIDFromAnchorOutput(ao, id) == chainID {
	// 		chainOutputID = id
	// 		chainOutput = ao.Clone().(*iotago.AnchorOutput)
	// 		break
	// 	}
	// }
	// if chainOutput == nil {
	// 	return nil, fmt.Errorf("unable to find UTXO for chain (%s) in owned UTXOs", chainID.String())
	// }

	// newConditions := make(iotago.AnchorOutputUnlockConditions, len(chainOutput.UnlockConditions))
	// for i, c := range chainOutput.UnlockConditions {
	// 	if _, ok := c.(*iotago.GovernorAddressUnlockCondition); ok {
	// 		// change the gov unlock condiiton to the new owner
	// 		newConditions[i] = &iotago.GovernorAddressUnlockCondition{
	// 			Address: newGovController,
	// 		}
	// 		continue
	// 	}
	// 	newConditions[i] = c
	// }
	// chainOutput.UnlockConditions = newConditions
	// chainOutput.AnchorID = chainID // in case right after mint where outputID is still 0

	// inputIDs := iotago.OutputIDs{chainOutputID}
	// outputs := iotago.TxEssenceOutputs{chainOutput}

	// return CreateAndSignTx(
	// 	wallet,
	// 	inputIDs.UTXOInputs(),
	// 	outputs,
	// 	creationSlot,
	// 	l1API,
	// )
}
