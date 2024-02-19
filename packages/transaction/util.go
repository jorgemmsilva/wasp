package transaction

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/iota.go/v4/builder"
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
	inputs iotago.OutputSet,
	remainder []iotago.Output,
	blockIssuerAccountID iotago.AccountID,
	err error,
) {
	inputs = make(iotago.OutputSet)
	sum := NewEmptyAssetsWithMana()

	unspentOutputIDs := lo.Keys(unspentOutputs)
	slices.SortFunc(unspentOutputIDs, func(a, b iotago.OutputID) int {
		return bytes.Compare(a[:], b[:])
	})
	var issuerAccountOutput *iotago.AccountOutput
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
		if accUTXO, ok := output.(*iotago.AccountOutput); ok {
			// NOTE: only the 1st account output found will be used
			// this is an UTXO that holds an account make note of it. it should be used to save all excess base tokens + mana
			if issuerAccountOutput != nil {
				continue // already have an account output
			}
			issuerAccountOutput = accUTXO
		}
		inputs[outputID] = output
		a, err2 := AssetsAndAvailableManaFromOutput(outputID, output, slotIndex, l1)
		if err2 != nil {
			return nil, nil, iotago.EmptyAccountID, err2
		}
		sum.Add(a)
		if sum.Geq(target) {
			break
		}
	}
	if issuerAccountOutput == nil {
		return nil, nil, iotago.EmptyAccountID, fmt.Errorf("no account issuer output found")
	}
	remainder, err = computeRemainderOutputs(senderAddress, issuerAccountOutput, sum, target, l1.APIForSlot(slotIndex))
	if err != nil {
		return nil, nil, iotago.EmptyAccountID, err
	}
	return inputs, remainder, issuerAccountOutput.AccountID, nil
}

// computeRemainderOutputs calculates remainders for base tokens and native tokens
// - available is what is available in inputs
// - target is what is in outputs, except the remainder output itself with its storage deposit
// Returns (nil, error) if inputs are not enough (taking into anchor storage deposit requirements)
// If return (nil, nil) it means remainder is a perfect match between inputs and outputs, remainder not needed
//

func computeRemainderOutputs(
	senderAddress iotago.Address,
	accountOutput *iotago.AccountOutput,
	available *AssetsWithMana,
	target *AssetsWithMana,
	l1API iotago.API,
) (ret []iotago.Output, err error) {
	excess := available.Clone()
	if excess.BaseTokens < target.BaseTokens {
		return nil, ErrNotEnoughBaseTokens
	}
	excess.BaseTokens -= target.BaseTokens

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
		sd, err2 := l1API.StorageScoreStructure().MinDeposit(out)
		if err2 != nil {
			return nil, err2
		}
		if excess.BaseTokens < sd {
			return nil, ErrNotEnoughBaseTokensForStorageDeposit
		}
		excess.BaseTokens -= sd
		out.Amount = sd
		ret = append(ret, out)
	}

	// NOTE: the account output must be the last one ( other tx building functions assume this is the case)
	// place all remaining base tokens in the account output
	newAccountOutput := accountOutput.Clone().(*iotago.AccountOutput)

	minSD, err := l1API.StorageScoreStructure().MinDeposit(newAccountOutput)
	if err != nil {
		return nil, err
	}
	if excess.BaseTokens < minSD {
		return nil, fmt.Errorf("not enough remaining base tokens for the Storage Deposit of the Account Output")
	}

	newAccountOutput.Amount = excess.BaseTokens
	newAccountOutput.Mana = 0 // let the mana be placed here when `AllotMinRequiredManaAndStoreRemainingManaInOutput` is called
	ret = append(ret, newAccountOutput)

	return ret, nil
}

func TxBuilderFromInputsAndOutputs(
	l1API iotago.API,
	inputs iotago.OutputSet,
	outputs []iotago.Output,
	wallet cryptolib.VariantKeyPair,
) *builder.TransactionBuilder {
	txBuilder := builder.NewTransactionBuilder(l1API, wallet)

	for inputID, input := range inputs {
		txBuilder.AddInput(&builder.TxInput{
			UnlockTarget: wallet.Address(),
			InputID:      inputID,
			Input:        input,
		})
	}

	for _, output := range outputs {
		txBuilder.AddOutput(output)
	}
	return txBuilder
}

func finalizeAndSignTx(
	txBuilder *builder.TransactionBuilder,
	blockIssuance *api.IssuanceBlockHeaderResponse,
	storedManaOutputIndex int,
	blockIssuerID iotago.AccountID,
) (*iotago.SignedTransaction, error) {
	txBuilder.AddCommitmentInput(&iotago.CommitmentInput{CommitmentID: lo.Must(blockIssuance.LatestCommitment.ID())})
	txBuilder.AddBlockIssuanceCreditInput(&iotago.BlockIssuanceCreditInput{AccountID: blockIssuerID})
	txBuilder.SetCreationSlot(blockIssuance.LatestCommitment.Slot)
	txBuilder.AllotMinRequiredManaAndStoreRemainingManaInOutput(
		txBuilder.CreationSlot(),
		blockIssuance.LatestCommitment.ReferenceManaCost,
		blockIssuerID,
		storedManaOutputIndex,
	)

	return txBuilder.Build()
}

func BlockFromTx(
	l1API iotago.API,
	bi *api.IssuanceBlockHeaderResponse,
	signedTx *iotago.SignedTransaction,
	blockIssuerID iotago.AccountID,
	signer cryptolib.VariantKeyPair,
) (*iotago.Block, error) {
	// the issuing time of the blocks need to be monotonically increasing
	issuingTime := time.Now().UTC()
	if bi.LatestParentBlockIssuingTime.After(issuingTime) {
		issuingTime = bi.LatestParentBlockIssuingTime.Add(time.Nanosecond)
	}

	return builder.NewBasicBlockBuilder(l1API).
		SlotCommitmentID(bi.LatestCommitment.MustID()).
		LatestFinalizedSlot(bi.LatestFinalizedSlot).
		StrongParents(bi.StrongParents).
		WeakParents(bi.WeakParents).
		ShallowLikeParents(bi.ShallowLikeParents).
		Payload(signedTx).
		CalculateAndSetMaxBurnedMana(bi.LatestCommitment.ReferenceManaCost).
		IssuingTime(issuingTime).
		SignWithSigner(blockIssuerID, signer, signer.Address()).
		Build()
}

func FinalizeTxAndBuildBlock(
	l1API iotago.API,
	txBuilder *builder.TransactionBuilder,
	blockIssuance *api.IssuanceBlockHeaderResponse,
	storedManaOutputIndex int,
	blockIssuerID iotago.AccountID,
	signer cryptolib.VariantKeyPair,
) (*iotago.Block, error) {
	tx, err := finalizeAndSignTx(txBuilder, blockIssuance, storedManaOutputIndex, blockIssuerID)
	if err != nil {
		return nil, err
	}
	return BlockFromTx(l1API, blockIssuance, tx, blockIssuerID, signer)
}
