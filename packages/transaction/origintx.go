package transaction

import (
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/origin"
	"github.com/iotaledger/wasp/packages/parameters"
)

// NewChainOriginTransaction creates new origin transaction for the self-governed chain
// returns the transaction and newly minted chain ID
func NewChainOriginTransaction(
	keyPair *cryptolib.KeyPair,
	stateControllerAddress iotago.Address,
	governanceControllerAddress iotago.Address,
	deposit uint64,
	initParams dict.Dict,
	unspentOutputs iotago.OutputSet,
	unspentOutputIDs iotago.OutputIDs,
) (*iotago.Transaction, *iotago.AliasOutput, isc.ChainID, error) {
	if len(unspentOutputs) != len(unspentOutputIDs) {
		panic("mismatched lengths of outputs and inputs slices")
	}

	walletAddr := keyPair.GetPublicKey().AsEd25519Address()

	if initParams == nil {
		initParams = dict.New()
	}

	// TODO this storage assumption stuff needs to go
	minSD := NewStorageDepositEstimate().AnchorOutput
	if deposit < minSD {
		deposit = minSD
	}

	aliasOutput := &iotago.AliasOutput{
		Amount:        deposit,
		StateMetadata: origin.L1Commitment(initParams, deposit-minSD).Bytes(),
		Conditions: iotago.UnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{Address: stateControllerAddress},
			&iotago.GovernorAddressUnlockCondition{Address: governanceControllerAddress},
		},
		Features: iotago.Features{
			&iotago.SenderFeature{
				Address: walletAddr,
			},
			&iotago.MetadataFeature{Data: initParams.Bytes()},
		},
	}

	// minSD := parameters.L1().Protocol.RentStructure.MinRent(aliasOutput)
	// if aliasOutput.Deposit() < minSD {
	// 	aliasOutput.Amount = minSD
	// 	aliasOutput.StateMetadata = origin.L1Commitment(initParams, minSD).Bytes()
	// }

	txInputs, remainderOutput, err := computeInputsAndRemainder(
		walletAddr,
		aliasOutput.Amount,
		nil,
		nil,
		unspentOutputs,
		unspentOutputIDs,
	)
	if err != nil {
		return nil, aliasOutput, isc.ChainID{}, err
	}
	outputs := iotago.Outputs{aliasOutput}
	if remainderOutput != nil {
		outputs = append(outputs, remainderOutput)
	}
	essence := &iotago.TransactionEssence{
		NetworkID: parameters.L1().Protocol.NetworkID(),
		Inputs:    txInputs.UTXOInputs(),
		Outputs:   outputs,
	}
	sigs, err := essence.Sign(
		txInputs.OrderedSet(unspentOutputs).MustCommitment(),
		keyPair.GetPrivateKey().AddressKeysForEd25519Address(walletAddr),
	)
	if err != nil {
		return nil, aliasOutput, isc.ChainID{}, err
	}
	tx := &iotago.Transaction{
		Essence: essence,
		Unlocks: MakeSignatureAndReferenceUnlocks(len(txInputs), sigs[0]),
	}
	txid, err := tx.ID()
	if err != nil {
		return nil, aliasOutput, isc.ChainID{}, err
	}
	chainID := isc.ChainIDFromAliasID(iotago.AliasIDFromOutputID(iotago.OutputIDFromTransactionIDAndIndex(txid, 0)))
	return tx, aliasOutput, chainID, nil
}
