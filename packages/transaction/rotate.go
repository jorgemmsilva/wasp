package transaction

import (
	"errors"
	"fmt"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/util"
)

func NewRotateChainStateControllerTx(
	aliasID iotago.AccountID,
	newStateController iotago.Address,
	chainOutputID iotago.OutputID,
	chainOutput iotago.Output,
	creationSlot iotago.SlotIndex,
	kp *cryptolib.KeyPair,
) (*iotago.SignedTransaction, error) {
	o, ok := chainOutput.(*iotago.AccountOutput)
	if !ok {
		return nil, fmt.Errorf("provided output is not the correct one. Expected AccountOutput, received %T=%v", chainOutput, chainOutput)
	}
	resolvedAccountID := util.AccountIDFromAccountOutput(o, chainOutputID)
	if resolvedAccountID != aliasID {
		return nil, fmt.Errorf("provided output is not the correct one. Expected ChainID: %s, got: %s",
			aliasID.ToAddress().Bech32(parameters.NetworkPrefix()),
			chainOutput.(*iotago.AccountOutput).AccountID.ToAddress().Bech32(parameters.NetworkPrefix()),
		)
	}

	// create a TX with that UTXO as input, and the updated addr unlock condition on the new output
	inputIDs := iotago.OutputIDs{chainOutputID}
	outSet := iotago.OutputSet{}
	outSet[chainOutputID] = chainOutput
	inputsCommitment := inputIDs.OrderedSet(outSet).MustCommitment(parameters.L1API())

	newChainOutput := chainOutput.Clone().(*iotago.AccountOutput)
	newChainOutput.AccountID = resolvedAccountID
	oldUnlockConditions := newChainOutput.UnlockConditionSet()
	newChainOutput.Conditions = make(iotago.AccountOutputUnlockConditions, len(oldUnlockConditions))

	// update the unlock conditions to the new state controller
	i := 0
	for t, condition := range oldUnlockConditions {
		newChainOutput.Conditions[i] = condition.Clone()
		if t == iotago.UnlockConditionStateControllerAddress {
			// found the condition to alter
			c, ok := newChainOutput.Conditions[i].(*iotago.StateControllerAddressUnlockCondition)
			if !ok {
				return nil, errors.New("unexpected error trying to get StateControllerAddressUnlockCondition")
			}
			c.Address = newStateController
			newChainOutput.Conditions[i] = c.Clone()
		}
		i++
	}

	// remove any "sender feature"
	var newFeatures iotago.AccountOutputFeatures
	for t, feature := range chainOutput.FeatureSet() {
		if t != iotago.FeatureSender {
			newFeatures = append(newFeatures, feature)
		}
	}
	newChainOutput.Features = newFeatures

	outputs := iotago.TxEssenceOutputs{newChainOutput}
	return CreateAndSignTx(
		kp,
		inputIDs.UTXOInputs(),
		inputsCommitment,
		outputs,
		creationSlot,
	)
}
