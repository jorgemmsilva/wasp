package transaction

import (
	"errors"
	"fmt"
	"math/big"

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

// computeInputsAndRemainder computes inputs and remainder for given outputs balances.
// Takes into account minimum storage deposit requirements
// The inputs are consumed one by one in the order provided in the parameters.
// Consumes only what is needed to cover output balances
// Returned reminder is nil if not needed
func ComputeInputsAndRemainder(
	senderAddress iotago.Address,
	baseTokenOut iotago.BaseToken,
	tokensOut iotago.NativeTokenSum,
	nftsOut map[iotago.NFTID]bool,
	unspentOutputs iotago.OutputSet,
	unspentOutputIDs iotago.OutputIDs,
) (
	inputIDs iotago.OutputIDs,
	remainder iotago.TxEssenceOutputs,
	err error,
) {
	baseTokensIn := iotago.BaseToken(0)
	tokensIn := iotago.NativeTokenSum{}
	nftsIn := make(map[iotago.NFTID]bool)

	// we need to start with a predefined error, otherwise we won't return a failure
	// even if not a single unspentOutputID was given and we never run into the following loop.
	errLast := errors.New("no valid inputs found to create transaction")

	for _, outputID := range unspentOutputIDs {
		output, ok := unspentOutputs[outputID]
		if !ok {
			return nil, nil, errors.New("computeInputsAndRemainder: outputID is not in the set")
		}

		if nftOutput, ok := output.(*iotago.NFTOutput); ok {
			nftID := util.NFTIDFromNFTOutput(nftOutput, outputID)
			if nftsOut[nftID] {
				nftsIn[nftID] = true
			} else {
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
		a := AssetsFromOutput(output)
		baseTokensIn += a.BaseTokens
		for _, nativeToken := range a.NativeTokens {
			nativeTokenAmountSum, ok := tokensIn[nativeToken.ID]
			if !ok {
				nativeTokenAmountSum = new(big.Int)
			}
			nativeTokenAmountSum.Add(nativeTokenAmountSum, nativeToken.Amount)
			tokensIn[nativeToken.ID] = nativeTokenAmountSum
		}
		// calculate remainder. It will return  err != nil if inputs not enough.
		remainder, errLast = computeRemainderOutputs(senderAddress, baseTokensIn, baseTokenOut, tokensIn, tokensOut)
		if errLast == nil && len(nftsIn) == len(nftsOut) {
			break
		}
	}
	if errLast != nil {
		return nil, nil, errLast
	}
	return inputIDs, remainder, nil
}

// computeRemainderOutputs calculates remainders for base tokens and native tokens
// - inBaseTokens and inTokens is what is available in inputs
// - outBaseTokens, outTokens is what is in outputs, except the remainder output itself with its storage deposit
// Returns (nil, error) if inputs are not enough (taking into account storage deposit requirements)
// If return (nil, nil) it means remainder is a perfect match between inputs and outputs, remainder not needed
//
//nolint:gocyclo
func computeRemainderOutputs(
	senderAddress iotago.Address,
	inBaseTokens, outBaseTokens iotago.BaseToken,
	inTokens, outTokens iotago.NativeTokenSum,
) (ret iotago.TxEssenceOutputs, err error) {
	if inBaseTokens < outBaseTokens {
		return nil, ErrNotEnoughBaseTokens
	}
	remBaseTokens := inBaseTokens - outBaseTokens

	// collect all token ids
	nativeTokenIDs := make(map[iotago.NativeTokenID]bool)
	for id := range inTokens {
		nativeTokenIDs[id] = true
	}
	for id := range outTokens {
		nativeTokenIDs[id] = true
	}
	remTokens := iotago.NativeTokenSum{}
	// calc remainders by outputs
	for nativeTokenID := range nativeTokenIDs {
		amountIn := inTokens.ValueOrBigInt0(nativeTokenID)
		amountOut := outTokens.ValueOrBigInt0(nativeTokenID)
		if amountIn.Cmp(amountOut) < 0 {
			return nil, ErrNotEnoughNativeTokens
		}
		diff := new(big.Int).Sub(amountIn, amountOut)
		if !util.IsZeroBigInt(diff) {
			remTokens[nativeTokenID] = diff
		}
	}

	for ntId, ntAmount := range remTokens {
		out := &iotago.BasicOutput{
			Features: iotago.BasicOutputFeatures{
				&iotago.NativeTokenFeature{ID: ntId, Amount: ntAmount},
			},
			Conditions: iotago.BasicOutputUnlockConditions{
				&iotago.AddressUnlockCondition{Address: senderAddress},
			},
		}
		sd, err := parameters.L1API().RentStructure().MinDeposit(out)
		if err != nil {
			return nil, err
		}
		if remBaseTokens < sd {
			return nil, ErrNotEnoughBaseTokensForStorageDeposit
		}
		remBaseTokens -= sd
		out.Amount = sd
		ret = append(ret, out)
	}

	if remBaseTokens > 0 {
		out := &iotago.BasicOutput{
			Amount: remBaseTokens,
			Conditions: iotago.BasicOutputUnlockConditions{
				&iotago.AddressUnlockCondition{Address: senderAddress},
			},
		}
		sd, err := parameters.L1API().RentStructure().MinDeposit(out)
		if err != nil {
			return nil, err
		}
		if remBaseTokens < sd {
			return nil, ErrNotEnoughBaseTokensForStorageDeposit
		}
		ret = append(ret, out)
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
	inputsCommitment []byte,
	outputs iotago.TxEssenceOutputs,
	creationSlot iotago.SlotIndex,
) (*iotago.SignedTransaction, error) {
	panic("TODO: is this still relevant?")
	// IMPORTANT: make sure inputs and outputs are correctly ordered before
	// signing, otherwise it might fail when it reaches the node, since the PoW
	// that would order the tx is done after the signing, so if we don't order
	// now, we might sign an invalid TX
	tx := &iotago.Transaction{
		API: parameters.L1API(),
		TransactionEssence: &iotago.TransactionEssence{
			NetworkID:    parameters.L1().Protocol.NetworkID(),
			CreationSlot: creationSlot,
			Inputs:       inputs,
		},
		Outputs: outputs,
	}

	sigs, err := tx.Sign(
		inputsCommitment,
		wallet.GetPrivateKey().AddressKeysForEd25519Address(wallet.Address()),
	)
	if err != nil {
		return nil, err
	}

	return &iotago.SignedTransaction{
		API:         parameters.L1API(),
		Transaction: tx,
		Unlocks:     MakeSignatureAndReferenceUnlocks(len(inputs), sigs[0]),
	}, nil
}
