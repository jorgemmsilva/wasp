package transaction

import (
	"fmt"
	"math"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/util"
)

// NewAccountOutputForStateController creates a new "block issuer account output" for the next committee (with minSD)
func NewAccountOutputForStateController(l1API iotago.API, stateControllerPubKey *cryptolib.PublicKey) *iotago.AccountOutput {
	accountOutput := &iotago.AccountOutput{
		Amount:         0,
		AccountID:      iotago.EmptyAccountID,
		FoundryCounter: 0,
		UnlockConditions: []iotago.AccountOutputUnlockCondition{
			&iotago.AddressUnlockCondition{
				Address: stateControllerPubKey.AsEd25519Address(),
			},
		},
		Features: []iotago.AccountOutputFeature{
			&iotago.BlockIssuerFeature{
				ExpirySlot: math.MaxUint32,
				BlockIssuerKeys: []iotago.BlockIssuerKey{
					iotago.Ed25519PublicKeyHashBlockIssuerKeyFromPublicKey(stateControllerPubKey.AsHiveEd25519PubKey()),
				},
			},
		},
	}
	accountOutput.Amount = lo.Must(l1API.StorageScoreStructure().MinDeposit(accountOutput))
	return accountOutput
}

func NewAccountOutputForStateControllerTx(
	unspentOutputs iotago.OutputSet,
	sender cryptolib.VariantKeyPair,
	stateControllerPubKey *cryptolib.PublicKey,
	l1APIProvider iotago.APIProvider,
	blockIssuance *api.IssuanceBlockHeaderResponse,
) (*iotago.Block, iotago.AccountID, error) {
	slot := blockIssuance.LatestCommitment.Slot
	l1API := l1APIProvider.APIForSlot(slot)

	accountOutput := NewAccountOutputForStateController(l1API, stateControllerPubKey)

	inputs, remainder, blockIssuerAccountID, err := ComputeInputsAndRemainder(
		sender.Address(),
		unspentOutputs,
		NewAssetsWithMana(isc.NewAssetsBaseTokens(accountOutput.Amount), 0),
		slot,
		l1APIProvider,
	)
	if err != nil {
		return nil, iotago.EmptyAccountID, err
	}

	outputs := []iotago.Output{accountOutput}
	outputs = append(outputs, remainder...)

	block, err := FinalizeTxAndBuildBlock(
		l1API,
		TxBuilderFromInputsAndOutputs(l1API, inputs, outputs, sender),
		blockIssuance,
		len(outputs)-1, // store mana in the last output
		blockIssuerAccountID,
		sender,
	)

	stateControllerAccountIssuerOutputID := iotago.OutputIDFromTransactionIDAndIndex(lo.Must(util.TxFromBlock(block).Transaction.ID()), 0)
	newAccountID := iotago.AccountIDFromOutputID(stateControllerAccountIssuerOutputID)

	return block, newAccountID, err
}

func NewRotateChainStateControllerTx(
	unspentOutputs iotago.OutputSet,
	sender cryptolib.VariantKeyPair,
	anchorID iotago.AnchorID,
	newStateController *cryptolib.PublicKey,
	chainOutputID iotago.OutputID,
	chainOutput iotago.Output,
	l1APIProvider iotago.APIProvider,
	blockIssuance *api.IssuanceBlockHeaderResponse,
) (*iotago.Block, error) {
	slot := blockIssuance.LatestCommitment.Slot
	l1API := l1APIProvider.APIForSlot(slot)

	o, ok := chainOutput.(*iotago.AnchorOutput)
	if !ok {
		return nil, fmt.Errorf("provided output is not the correct one. Expected AnchorOutput, received %T=%v", chainOutput, chainOutput)
	}
	resolvedAnchorID := util.AnchorIDFromAnchorOutput(o, chainOutputID)
	if resolvedAnchorID != anchorID {
		return nil, fmt.Errorf("provided output is not the correct one. Expected ChainID: %s, got: %s",
			anchorID.ToHex(),
			chainOutput.(*iotago.AnchorOutput).AnchorID.ToHex(),
		)
	}

	// create a TX with that UTXO as input, and the updated addr unlock condition on the new output
	// outSet := iotago.OutputSet{}
	// outSet[chainOutputID] = chainOutput

	newChainOutput := chainOutput.Clone().(*iotago.AnchorOutput)
	newChainOutput.AnchorID = resolvedAnchorID
	// oldUnlockConditions := newChainOutput.UnlockConditionSet()
	// newChainOutput.UnlockConditions = make(iotago.AnchorOutputUnlockConditions, len(oldUnlockConditions))
	newChainOutput.UnlockConditions.Upsert(
		&iotago.StateControllerAddressUnlockCondition{
			Address: newStateController.AsEd25519Address(),
		},
	)

	// TODO remove, upsert should be enough
	// // update the unlock conditions to the new state controller
	// i := 0
	// for t, condition := range oldUnlockConditions {
	// 	newChainOutput.UnlockConditions[i] = condition.Clone()
	// 	if t == iotago.UnlockConditionStateControllerAddress {
	// 		// found the condition to alter
	// 		c, ok := newChainOutput.UnlockConditions[i].(*iotago.StateControllerAddressUnlockCondition)
	// 		if !ok {
	// 			return nil, errors.New("unexpected error trying to get StateControllerAddressUnlockCondition")
	// 		}
	// 		c.Address = newStateController
	// 		newChainOutput.UnlockConditions[i] = c.Clone()
	// 	}
	// 	i++
	// }

	// remove any "sender feature"
	var newFeatures iotago.AnchorOutputFeatures
	for t, feature := range chainOutput.FeatureSet() {
		if t != iotago.FeatureSender {
			newFeatures = append(newFeatures, feature)
		}
	}
	newChainOutput.Features = newFeatures

	// create an isser account for the next state controller
	accountOutput := NewAccountOutputForStateController(l1API, newStateController)

	inputs, remainder, blockIssuerAccountID, err := ComputeInputsAndRemainder(
		sender.Address(),
		unspentOutputs,
		NewAssetsWithMana(isc.NewAssetsBaseTokens(accountOutput.Amount), 0),
		slot,
		l1APIProvider,
	)
	if err != nil {
		return nil, err
	}

	outputs := []iotago.Output{accountOutput, newChainOutput}
	outputs = append(outputs, remainder...)

	inputs[chainOutputID] = chainOutput

	return FinalizeTxAndBuildBlock(
		l1API,
		TxBuilderFromInputsAndOutputs(l1API, inputs, outputs, sender),
		blockIssuance,
		len(outputs)-1, // store mana in the last output
		blockIssuerAccountID,
		sender,
	)
}
