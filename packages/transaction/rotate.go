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
	anchorID iotago.AnchorID,
	newStateController iotago.Address,
	chainOutputID iotago.OutputID,
	chainOutput iotago.Output,
	creationSlot iotago.SlotIndex,
	kp *cryptolib.KeyPair,
) (*iotago.SignedTransaction, error) {
	o, ok := chainOutput.(*iotago.AnchorOutput)
	if !ok {
		return nil, fmt.Errorf("provided output is not the correct one. Expected AnchorOutput, received %T=%v", chainOutput, chainOutput)
	}
	resolvedAnchorID := util.AnchorIDFromAnchorOutput(o, chainOutputID)
	if resolvedAnchorID != anchorID {
		return nil, fmt.Errorf("provided output is not the correct one. Expected ChainID: %s, got: %s",
			anchorID.ToAddress().Bech32(parameters.NetworkPrefix()),
			chainOutput.(*iotago.AnchorOutput).AnchorID.ToAddress().Bech32(parameters.NetworkPrefix()),
		)
	}

	// create a TX with that UTXO as input, and the updated addr unlock condition on the new output
	inputIDs := iotago.OutputIDs{chainOutputID}
	outSet := iotago.OutputSet{}
	outSet[chainOutputID] = chainOutput

	newChainOutput := chainOutput.Clone().(*iotago.AnchorOutput)
	newChainOutput.AnchorID = resolvedAnchorID
	oldUnlockConditions := newChainOutput.UnlockConditionSet()
	newChainOutput.Conditions = make(iotago.AnchorOutputUnlockConditions, len(oldUnlockConditions))

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
	var newFeatures iotago.AnchorOutputFeatures
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
		outputs,
		creationSlot,
	)
}
