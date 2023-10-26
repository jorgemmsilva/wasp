package transaction

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"slices"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/util"
)

var ErrNoAccountOutputAtIndex0 = errors.New("origin AccountOutput not found at index 0")

// GetAnchorFromTransaction analyzes the output at index 0 and extracts anchor information. Otherwise error
func GetAnchorFromTransaction(tx *iotago.Transaction) (*isc.StateAnchor, *iotago.AccountOutput, error) {
	anchorOutput, ok := tx.Outputs[0].(*iotago.AccountOutput)
	if !ok {
		return nil, nil, ErrNoAccountOutputAtIndex0
	}
	txid, err := tx.ID()
	if err != nil {
		return nil, anchorOutput, fmt.Errorf("GetAnchorFromTransaction: %w", err)
	}
	accountID := anchorOutput.AccountID
	isOrigin := false

	if accountID.Empty() {
		isOrigin = true
		accountID = iotago.AccountIDFromOutputID(iotago.OutputIDFromTransactionIDAndIndex(txid, 0))
	}
	return &isc.StateAnchor{
		IsOrigin:             isOrigin,
		OutputID:             iotago.OutputIDFromTransactionIDAndIndex(txid, 0),
		ChainID:              isc.ChainIDFromAccountID(accountID),
		StateController:      anchorOutput.StateController(),
		GovernanceController: anchorOutput.GovernorAddress(),
		StateIndex:           anchorOutput.StateIndex,
		StateData:            anchorOutput.StateMetadata,
		Deposit:              anchorOutput.Amount,
	}, anchorOutput, nil
}

// ComputeInputsAndRemainder finds inputs so that the neededAssets are covered,
// and computes remainder outputs, taking into account minimum storage deposit requirements.
// The inputs are consumed one by one in deterministic order (sorted by OutputID).
func ComputeInputsAndRemainder(
	senderAddress iotago.Address,
	unspentOutputs iotago.OutputSet,
	target *isc.AssetsWithMana,
	slotIndex iotago.SlotIndex,
) (
	inputIDs iotago.OutputIDs,
	remainder iotago.TxEssenceOutputs,
	err error,
) {
	sum := isc.NewEmptyAssetsWithMana()

	unspentOutputIDs := lo.Keys(unspentOutputs)
	slices.SortFunc(unspentOutputIDs, func(a, b iotago.OutputID) int {
		return bytes.Compare(a[:], b[:])
	})
	for _, outputID := range unspentOutputIDs {
		output := unspentOutputs[outputID]
		if nftOutput, ok := output.(*iotago.NFTOutput); ok {
			nftID := util.NFTIDFromNFTOutput(nftOutput, outputID)
			if !lo.Contains(target.NFTs, nftID) {
				// this is an UTXO that holds an NFT that is not relevant for this tx, should be skipped
				continue
			}
		}
		if _, ok := output.(*iotago.AccountOutput); ok {
			// this is an UTXO that holds an account that is not relevant for this tx, should be skipped
			continue
		}
		if _, ok := output.(*iotago.FoundryOutput); ok {
			// this is an UTXO that holds an foundry that is not relevant for this tx, should be skipped
			continue
		}
		if output.UnlockConditionSet().StorageDepositReturn() != nil {
			// don't consume anything with SDRUC
			continue
		}
		inputIDs = append(inputIDs, outputID)
		a, err := AssetsAndManaFromOutput(outputID, output, slotIndex)
		if err != nil {
			return nil, nil, err
		}
		sum.Add(a)
		if sum.Geq(target) {
			break
		}
	}
	remainder, err = computeRemainderOutputs(senderAddress, sum, target)
	if err != nil {
		return nil, nil, err
	}
	return inputIDs, remainder, nil
}

// computeRemainderOutputs calculates remainders for base tokens and native tokens
// - available is what is available in inputs
// - target is what is in outputs, except the remainder output itself with its storage deposit
// Returns (nil, error) if inputs are not enough (taking into account storage deposit requirements)
// If return (nil, nil) it means remainder is a perfect match between inputs and outputs, remainder not needed
//
//nolint:gocyclo
func computeRemainderOutputs(
	senderAddress iotago.Address,
	available *isc.AssetsWithMana,
	target *isc.AssetsWithMana,
) (ret iotago.TxEssenceOutputs, err error) {
	if available.BaseTokens < target.BaseTokens {
		return nil, ErrNotEnoughBaseTokens
	}
	excess := *isc.NewEmptyAssetsWithMana()
	excess.BaseTokens = available.BaseTokens - target.BaseTokens

	if available.Mana < target.Mana {
		return nil, ErrNotEnoughMana
	}
	excess.Mana = available.Mana - target.Mana

	availableNTs := available.NativeTokenSum()
	for _, nt := range available.NativeTokens {
		excess.AddNativeTokens(nt.ID, nt.Amount)
	}
	for _, nt := range target.NativeTokens {
		if availableNTs.ValueOrBigInt0(nt.ID).Cmp(nt.Amount) < 0 {
			return nil, ErrNotEnoughNativeTokens
		}
		amount := new(big.Int).Set(nt.Amount)
		excess.AddNativeTokens(nt.ID, amount.Neg(amount))
	}

	for _, nt := range excess.NativeTokens {
		if nt.Amount.Sign() == 0 {
			continue
		}
		out := &iotago.BasicOutput{
			Features: iotago.BasicOutputFeatures{
				&iotago.NativeTokenFeature{ID: nt.ID, Amount: nt.Amount},
			},
			Conditions: iotago.BasicOutputUnlockConditions{
				&iotago.AddressUnlockCondition{Address: senderAddress},
			},
		}
		sd, err := parameters.Storage().MinDeposit(out)
		if err != nil {
			return nil, err
		}
		if excess.BaseTokens < sd {
			return nil, ErrNotEnoughBaseTokensForStorageDeposit
		}
		excess.BaseTokens -= sd
		out.Amount = sd
		ret = append(ret, out)
	}

	if excess.BaseTokens > 0 {
		out := &iotago.BasicOutput{
			Amount: excess.BaseTokens,
			Conditions: iotago.BasicOutputUnlockConditions{
				&iotago.AddressUnlockCondition{Address: senderAddress},
			},
		}
		sd, err := parameters.Storage().MinDeposit(out)
		if err != nil {
			return nil, err
		}
		if excess.BaseTokens < sd {
			return nil, ErrNotEnoughBaseTokensForStorageDeposit
		}
		ret = append(ret, out)
	}

	if excess.Mana > 0 {
		if len(ret) == 0 {
			return nil, ErrNotEnoughBaseTokensForStorageDeposit
		}
		out := ret[len(ret)-1].(*iotago.BasicOutput)
		out.Mana = excess.Mana
	}

	return ret, nil
}

func MakeSignatureAndReferenceUnlocks(totalInputs int, sig iotago.Signature) iotago.Unlocks {
	ret := make(iotago.Unlocks, totalInputs)
	for i := range ret {
		if i == 0 {
			ret[0] = &iotago.SignatureUnlock{Signature: sig}
			continue
		}
		ret[i] = &iotago.ReferenceUnlock{Reference: 0}
	}
	return ret
}

func MakeSignatureAndAliasUnlockFeatures(totalInputs int, sig iotago.Signature) iotago.Unlocks {
	ret := make(iotago.Unlocks, totalInputs)
	for i := range ret {
		if i == 0 {
			ret[0] = &iotago.SignatureUnlock{Signature: sig}
			continue
		}
		ret[i] = &iotago.AccountUnlock{Reference: 0}
	}
	return ret
}

func MakeAnchorTransaction(tx *iotago.Transaction, sig iotago.Signature) *iotago.SignedTransaction {
	return &iotago.SignedTransaction{
		API:         parameters.L1API(),
		Transaction: tx,
		Unlocks:     MakeSignatureAndAliasUnlockFeatures(len(tx.TransactionEssence.Inputs), sig),
	}
}

func CreateAndSignTx(
	wallet *cryptolib.KeyPair,
	inputs iotago.TxEssenceInputs,
	outputs iotago.TxEssenceOutputs,
	creationSlot iotago.SlotIndex,
) (*iotago.SignedTransaction, error) {
	tx := &iotago.Transaction{
		API: parameters.L1API(),
		TransactionEssence: &iotago.TransactionEssence{
			NetworkID:    parameters.L1().Protocol.NetworkID(),
			CreationSlot: creationSlot,
			Inputs:       inputs,
		},
		Outputs: outputs,
	}

	sigs, err := tx.Sign(wallet.GetPrivateKey().AddressKeysForEd25519Address(wallet.Address()))
	if err != nil {
		return nil, err
	}

	return &iotago.SignedTransaction{
		API:         parameters.L1API(),
		Transaction: tx,
		Unlocks:     MakeSignatureAndReferenceUnlocks(len(inputs), sigs[0]),
	}, nil
}
