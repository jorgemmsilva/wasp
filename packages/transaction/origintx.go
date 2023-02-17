package transaction

import (
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/state"
)

// NewChainOriginTransaction creates new origin transaction for the self-governed chain
// returns the transaction and newly minted chain ID
func NewChainOriginTransaction(
	keyPair *cryptolib.KeyPair,
	stateControllerAddress iotago.Address,
	governanceControllerAddress iotago.Address,
	deposit uint64,
	unspentOutputs iotago.OutputSet,
	unspentOutputIDs iotago.OutputIDs,
) (*iotago.Transaction, isc.ChainID, error) {
	if len(unspentOutputs) != len(unspentOutputIDs) {
		panic("mismatched lengths of outputs and inputs slices")
	}

	walletAddr := keyPair.GetPublicKey().AsEd25519Address()

	aliasOutput := &iotago.AliasOutput{
		Amount:        deposit,
		StateMetadata: state.OriginL1Commitment().Bytes(),
		Conditions: iotago.UnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{Address: stateControllerAddress},
			&iotago.GovernorAddressUnlockCondition{Address: governanceControllerAddress},
		},
		Features: iotago.Features{
			&iotago.SenderFeature{
				Address: walletAddr,
			},
		},
	}
	{
		aliasStorageDeposit := NewStorageDepositEstimate().AnchorOutput
		if aliasOutput.Amount < aliasStorageDeposit {
			aliasOutput.Amount = aliasStorageDeposit
		}
	}
	txInputs, remainderOutput, err := computeInputsAndRemainder(
		walletAddr,
		aliasOutput.Amount,
		nil,
		nil,
		unspentOutputs,
		unspentOutputIDs,
	)
	if err != nil {
		return nil, isc.ChainID{}, err
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
		return nil, isc.ChainID{}, err
	}
	tx := &iotago.Transaction{
		Essence: essence,
		Unlocks: MakeSignatureAndReferenceUnlocks(len(txInputs), sigs[0]),
	}
	txid, err := tx.ID()
	if err != nil {
		return nil, isc.ChainID{}, err
	}
	chainID := isc.ChainIDFromAliasID(iotago.AliasIDFromOutputID(iotago.OutputIDFromTransactionIDAndIndex(txid, 0)))
	return tx, chainID, nil
}
