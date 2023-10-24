package transaction

import (
	"fmt"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/util"
)

func NewChangeGovControllerTx(
	chainID iotago.AccountID,
	newGovController iotago.Address,
	utxos iotago.OutputSet,
	wallet *cryptolib.KeyPair,
	creationSlot iotago.SlotIndex,
) (*iotago.SignedTransaction, error) {
	// find the correct chain UTXO
	var chainOutput *iotago.AccountOutput
	var chainOutputID iotago.OutputID
	for id, o := range utxos {
		ao, ok := o.(*iotago.AccountOutput)
		if !ok {
			continue
		}
		if util.AccountIDFromAccountOutput(ao, id) == chainID {
			chainOutputID = id
			chainOutput = ao.Clone().(*iotago.AccountOutput)
			break
		}
	}
	if chainOutput == nil {
		return nil, fmt.Errorf("unable to find UTXO for chain (%s) in owned UTXOs", chainID.String())
	}

	newConditions := make(iotago.AccountOutputUnlockConditions, len(chainOutput.Conditions))
	for i, c := range chainOutput.Conditions {
		if _, ok := c.(*iotago.GovernorAddressUnlockCondition); ok {
			// change the gov unlock condiiton to the new owner
			newConditions[i] = &iotago.GovernorAddressUnlockCondition{
				Address: newGovController,
			}
			continue
		}
		newConditions[i] = c
	}
	chainOutput.Conditions = newConditions
	chainOutput.AccountID = chainID // in case right after mint where outputID is still 0

	inputIDs := iotago.OutputIDs{chainOutputID}
	inputsCommitment := inputIDs.OrderedSet(utxos).MustCommitment(parameters.L1API())
	outputs := iotago.TxEssenceOutputs{chainOutput}

	return CreateAndSignTx(
		wallet,
		inputIDs.UTXOInputs(),
		inputsCommitment,
		outputs,
		creationSlot,
	)
}
