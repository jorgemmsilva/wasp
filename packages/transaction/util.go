package transaction

import (
	"bytes"
	"errors"
	"fmt"
	"slices"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"

	"github.com/iotaledger/wasp/packages/util"
)

var ErrNoAnchorOutputAtIndex0 = errors.New("origin AnchorOutput not found at index 0")

// GetAnchorFromTransaction analyzes the output at index 0 and extracts anchor information. Otherwise error
func GetAnchorFromTransaction(tx *iotago.Transaction) (*isc.StateAnchor, *iotago.AnchorOutput, error) {
	anchorOutput, ok := tx.Outputs[0].(*iotago.AnchorOutput)
	if !ok {
		return nil, nil, ErrNoAnchorOutputAtIndex0
	}
	txid, err := tx.ID()
	if err != nil {
		return nil, anchorOutput, fmt.Errorf("GetAnchorFromTransaction: %w", err)
	}
	anchorID := anchorOutput.AnchorID
	isOrigin := false

	if anchorID.Empty() {
		isOrigin = true
		anchorID = iotago.AnchorIDFromOutputID(iotago.OutputIDFromTransactionIDAndIndex(txid, 0))
	}
	stateData, err := StateMetadataBytesFromAnchorOutput(anchorOutput)
	if err != nil {
		return nil, nil, err
	}
	return &isc.StateAnchor{
		IsOrigin:             isOrigin,
		OutputID:             iotago.OutputIDFromTransactionIDAndIndex(txid, 0),
		ChainID:              isc.ChainIDFromAnchorID(anchorID),
		StateController:      anchorOutput.StateController(),
		GovernanceController: anchorOutput.GovernorAddress(),
		StateIndex:           anchorOutput.StateIndex,
		StateData:            stateData,
		Deposit:              anchorOutput.Amount,
	}, anchorOutput, nil
}

// ComputeInputsAndRemainder finds inputs so that the neededAssets are covered,
// and computes remainder outputs, taking into anchor minimum storage deposit requirements.
// The inputs are consumed one by one in deterministic order (sorted by OutputID).
func ComputeInputsAndRemainder(
	senderAddress iotago.Address,
	unspentOutputs iotago.OutputSet,
	target *AssetsWithMana,
	slotIndex iotago.SlotIndex,
	l1 iotago.APIProvider,
) (
	inputIDs iotago.OutputIDs,
	remainder iotago.TxEssenceOutputs,
	err error,
) {
	sum := NewEmptyAssetsWithMana()

	unspentOutputIDs := lo.Keys(unspentOutputs)
	slices.SortFunc(unspentOutputIDs, func(a, b iotago.OutputID) int {
		return bytes.Compare(a[:], b[:])
	})
	for _, outputID := range unspentOutputIDs {
		output := unspentOutputs[outputID]
		if output.UnlockConditionSet().StorageDepositReturn() != nil {
			// don't consume anything with SDRUC
			continue
		}
		if nftOutput, ok := output.(*iotago.NFTOutput); ok {
			nftID := util.NFTIDFromNFTOutput(nftOutput, outputID)
			if !lo.Contains(target.NFTs, nftID) {
				// this is an UTXO that holds an NFT that is not relevant for this tx, should be skipped
				continue
			}
		}
		if _, ok := output.(*iotago.AnchorOutput); ok {
			// this is an UTXO that holds an anchor that is not relevant for this tx, should be skipped
			continue
		}
		if _, ok := output.(*iotago.FoundryOutput); ok {
			// this is an UTXO that holds an foundry that is not relevant for this tx, should be skipped
			continue
		}
		if _, ok := output.(*iotago.AccountOutput); ok {
			// this is an UTXO that holds an account that is not relevant for this tx, should be skipped
			continue
		}
		inputIDs = append(inputIDs, outputID)
		a, err := AssetsAndAvailableManaFromOutput(outputID, output, slotIndex, l1)
		if err != nil {
			return nil, nil, err
		}
		sum.Add(a)
		if sum.Geq(target) {
			break
		}
	}
	remainder, err = computeRemainderOutputs(senderAddress, sum, target, l1.APIForSlot(slotIndex))
	if err != nil {
		return nil, nil, err
	}
	return inputIDs, remainder, nil
}

// computeRemainderOutputs calculates remainders for base tokens and native tokens
// - available is what is available in inputs
// - target is what is in outputs, except the remainder output itself with its storage deposit
// Returns (nil, error) if inputs are not enough (taking into anchor storage deposit requirements)
// If return (nil, nil) it means remainder is a perfect match between inputs and outputs, remainder not needed
//

func computeRemainderOutputs(
	senderAddress iotago.Address,
	available *AssetsWithMana,
	target *AssetsWithMana,
	l1API iotago.API,
) (ret iotago.TxEssenceOutputs, err error) {
	excess := available.Clone()
	if excess.BaseTokens < target.BaseTokens {
		return nil, ErrNotEnoughBaseTokens
	}
	excess.BaseTokens -= target.BaseTokens

	if excess.Mana < target.Mana {
		return nil, ErrNotEnoughMana
	}
	excess.Mana -= target.Mana

	for id, amount := range target.NativeTokens {
		excessAmount := excess.NativeTokens.ValueOrBigInt0(id)
		if excessAmount.Cmp(amount) < 0 {
			return nil, ErrNotEnoughNativeTokens
		}
		excess.NativeTokens[id] = excessAmount.Sub(excessAmount, amount)
		if excessAmount.Sign() == 0 {
			delete(excess.NativeTokens, id)
		}
	}

	for _, id := range excess.NativeTokenIDsSorted() {
		amount := excess.NativeTokens[id]
		out := &iotago.BasicOutput{
			Features: iotago.BasicOutputFeatures{
				&iotago.NativeTokenFeature{ID: id, Amount: amount},
			},
			UnlockConditions: iotago.BasicOutputUnlockConditions{
				&iotago.AddressUnlockCondition{Address: senderAddress},
			},
		}
		sd, err := l1API.StorageScoreStructure().MinDeposit(out)
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
			UnlockConditions: iotago.BasicOutputUnlockConditions{
				&iotago.AddressUnlockCondition{Address: senderAddress},
			},
		}
		sd, err := l1API.StorageScoreStructure().MinDeposit(out)
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
			// cannot place excess mana into a remainder output
			return nil, ErrExcessMana
		}
		out := ret[len(ret)-1].(*iotago.BasicOutput)
		out.Mana = excess.Mana
	}

	return ret, nil
}

func MakeSignatureAndReferenceUnlocks(totalInputs int, sig iotago.Signature) iotago.Unlocks {
	ret := make(iotago.Unlocks, totalInputs)
	ret[0] = &iotago.SignatureUnlock{Signature: sig}
	for i := 1; i < totalInputs; i++ {
		ret[i] = &iotago.ReferenceUnlock{Reference: 0}
	}
	return ret
}

func CreateAndSignTx(
	wallet *cryptolib.KeyPair,
	inputs iotago.TxEssenceInputs,
	outputs iotago.TxEssenceOutputs,
	creationSlot iotago.SlotIndex,
	l1API iotago.API,
) (*iotago.SignedTransaction, error) {
	tx := &iotago.Transaction{
		API: l1API,
		TransactionEssence: &iotago.TransactionEssence{
			NetworkID:    l1API.ProtocolParameters().NetworkID(),
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
		API:         l1API,
		Transaction: tx,
		Unlocks:     MakeSignatureAndReferenceUnlocks(len(inputs), sigs[0]),
	}, nil
}
