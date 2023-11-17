package rotate

import (
	"time"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
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

// used by the VM to create a rotatation tx after a rotation request
func MakeRotationTransactionForSelfManagedChain(
	nextAddr iotago.Address,
	chainInputs *isc.ChainOutputs,
	creationSlot iotago.SlotIndex,
) (*iotago.Transaction, iotago.Unlocks, error) {
	// The Account output cannot be consumed on this transaction (it is not a state transaction)
	anchorOutput := chainInputs.AnchorOutput.Clone().(*iotago.AnchorOutput)
	anchorOutput.UnlockConditions = iotago.AnchorOutputUnlockConditions{
		&iotago.StateControllerAddressUnlockCondition{Address: nextAddr},
		&iotago.GovernorAddressUnlockCondition{Address: nextAddr},
	}

	inputs := iotago.TxEssenceInputs{
		chainInputs.AnchorOutputID.UTXOInput(),
	}

	unlocks := iotago.Unlocks{
		&iotago.SignatureUnlock{}, // to be filled with the actual signature
	}

	tx := &iotago.Transaction{
		API: parameters.L1API(),
		TransactionEssence: &iotago.TransactionEssence{
			NetworkID:    parameters.L1().Protocol.NetworkID(),
			Inputs:       inputs,
			CreationSlot: creationSlot,
		},
		Outputs: iotago.TxEssenceOutputs{
			anchorOutput,
		},
	}

	return tx, unlocks, nil
}
