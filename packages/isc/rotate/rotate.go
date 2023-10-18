package rotate

import (
	"time"

	"github.com/iotaledger/hive.go/crypto/identity"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/parameters"
)

// IsRotateStateControllerRequest determines if request may be a committee rotation request
func IsRotateStateControllerRequest(req isc.Calldata) bool {
	target := req.CallTarget()
	return target.Contract == coreutil.CoreContractGovernanceHname && target.EntryPoint == coreutil.CoreEPRotateStateControllerHname
}

func NewRotateRequestOffLedger(chainID isc.ChainID, newStateAddress iotago.Address, keyPair *cryptolib.KeyPair, gasBudget uint64) isc.Request {
	args := dict.New()
	args.Set(coreutil.ParamStateControllerAddress, codec.EncodeAddress(newStateAddress))
	nonce := uint64(time.Now().UnixNano())
	ret := isc.NewOffLedgerRequest(chainID, coreutil.CoreContractGovernanceHname, coreutil.CoreEPRotateStateControllerHname, args, nonce, gasBudget)
	return ret.Sign(keyPair)
}

func MakeRotateStateControllerTransaction(
	nextAddr iotago.Address,
	chainInput *isc.AccountOutputWithID,
	creationSlot iotago.SlotIndex,
	accessPledge, consensusPledge identity.ID,
) (*iotago.Transaction, error) {
	output := chainInput.GetAccountOutput().Clone().(*iotago.AccountOutput)
	for i := range output.Conditions {
		if _, ok := output.Conditions[i].(*iotago.StateControllerAddressUnlockCondition); ok {
			output.Conditions[i] = &iotago.StateControllerAddressUnlockCondition{Address: nextAddr}
		}
		// TODO: it is probably not the correct way to do the governance transition
		if _, ok := output.Conditions[i].(*iotago.GovernorAddressUnlockCondition); ok {
			output.Conditions[i] = &iotago.GovernorAddressUnlockCondition{Address: nextAddr}
		}
	}

	// remove any "sender feature"
	var newFeatures iotago.AccountOutputFeatures
	for t, feature := range chainInput.GetAccountOutput().FeatureSet() {
		if t != iotago.FeatureSender {
			newFeatures = append(newFeatures, feature)
		}
	}
	output.Features = newFeatures

	result := &iotago.Transaction{
		API: parameters.L1API(),
		TransactionEssence: &iotago.TransactionEssence{
			NetworkID:    parameters.L1().Protocol.NetworkID(),
			Inputs:       iotago.TxEssenceInputs{chainInput.OutputID().UTXOInput()},
			CreationSlot: creationSlot,
		},
		Outputs: iotago.TxEssenceOutputs{output},
	}
	inputsCommitment := iotago.TxEssenceOutputs{chainInput.GetAccountOutput()}.
		MustCommitment(parameters.L1API())
	copy(result.InputsCommitment[:], inputsCommitment)
	return result, nil
}
