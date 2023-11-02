package transaction

import (
	"fmt"
	"math/big"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/util"
)

// NewTransferTransaction creates a basic output transaction that sends L1 Token to another L1 address
func NewTransferTransaction(
	baseTokens iotago.BaseToken,
	nativeToken *isc.NativeTokenAmount,
	mana iotago.Mana,
	senderAddress iotago.Address,
	senderKeyPair *cryptolib.KeyPair,
	targetAddress iotago.Address,
	unspentOutputs iotago.OutputSet,
	unlockConditions []iotago.UnlockCondition,
	creationSlot iotago.SlotIndex,
	disableAutoAdjustStorageDeposit bool, // if true, the minimal storage deposit won't be adjusted automatically
) (*iotago.SignedTransaction, error) {
	output := MakeBasicOutput(
		targetAddress,
		senderAddress,
		baseTokens,
		nativeToken,
		mana,
		nil,
		unlockConditions,
	)
	if !disableAutoAdjustStorageDeposit {
		output = AdjustToMinimumStorageDeposit(output)
	}

	storageDeposit, err := parameters.Storage().MinDeposit(output)
	if err != nil {
		return nil, err
	}
	if output.BaseTokenAmount() < storageDeposit {
		return nil, fmt.Errorf("%v: available %d < required %d base tokens",
			ErrNotEnoughBaseTokensForStorageDeposit, output.BaseTokenAmount(), storageDeposit)
	}

	assets := isc.NewAssets(baseTokens, nil)
	if nativeToken != nil {
		assets.AddNativeTokens(nativeToken.ID, nativeToken.Amount)
	}
	inputIDs, remainder, err := ComputeInputsAndRemainder(
		senderAddress,
		unspentOutputs,
		NewAssetsWithMana(assets, mana),
		creationSlot,
	)
	if err != nil {
		return nil, err
	}

	outputs := append(iotago.TxEssenceOutputs{output}, remainder...)

	return CreateAndSignTx(
		senderKeyPair,
		inputIDs.UTXOInputs(),
		outputs,
		creationSlot,
	)
}

// NewRequestTransaction creates a transaction including one or more requests to a chain.
// Empty assets in the request data defaults to 1 base token, which later is adjusted to the minimum storage deposit
// Assumes all unspentOutputs and corresponding unspentOutputIDs can be used as inputs, i.e. are
// unlockable for the sender address
func NewRequestTransaction(
	senderKeyPair *cryptolib.KeyPair,
	senderAddress iotago.Address, // might be different from the senderKP address (when sending as NFT or anchor)
	unspentOutputs iotago.OutputSet,
	request *isc.RequestParameters,
	nft *isc.NFT,
	creationSlot iotago.SlotIndex,
	disableAutoAdjustStorageDeposit bool, // if true, the minimal storage deposit won't be adjusted automatically
) (*iotago.SignedTransaction, error) {
	outputs := iotago.TxEssenceOutputs{}

	out := MakeRequestTransactionOutput(senderAddress, request, nft)
	if !disableAutoAdjustStorageDeposit {
		out = AdjustToMinimumStorageDeposit(out)
	}

	storageDeposit, err := parameters.Storage().MinDeposit(out)
	if err != nil {
		return nil, err
	}
	if out.BaseTokenAmount() < storageDeposit {
		return nil, fmt.Errorf("%v: available %d < required %d base tokens",
			ErrNotEnoughBaseTokensForStorageDeposit, out.BaseTokenAmount(), storageDeposit)
	}
	outputs = append(outputs, out)
	outputAssets := NewAssetsWithMana(AssetsFromOutput(out), out.StoredMana())

	outputs, outputAssets = updateOutputsWhenSendingOnBehalfOf(
		senderKeyPair,
		senderAddress,
		unspentOutputs,
		outputs,
		outputAssets,
		creationSlot,
	)

	inputIDs, remainder, err := ComputeInputsAndRemainder(
		senderKeyPair.Address(),
		unspentOutputs,
		outputAssets,
		creationSlot,
	)
	if err != nil {
		return nil, err
	}

	outputs = append(outputs, remainder...)

	return CreateAndSignTx(
		senderKeyPair,
		inputIDs.UTXOInputs(),
		outputs,
		creationSlot,
	)
}

func MakeRequestTransactionOutput(
	senderAddress iotago.Address,
	req *isc.RequestParameters,
	nft *isc.NFT,
) iotago.Output {
	assets := req.Assets
	if assets == nil {
		assets = isc.NewEmptyAssets()
	}

	var out iotago.Output
	out = MakeBasicOutput(
		req.TargetAddress,
		senderAddress,
		assets.BaseTokens,
		MustSingleNativeToken(assets.FungibleTokens),
		0,
		&isc.RequestMetadata{
			SenderContract: isc.EmptyContractIdentity(),
			TargetContract: req.Metadata.TargetContract,
			EntryPoint:     req.Metadata.EntryPoint,
			Params:         req.Metadata.Params,
			Allowance:      req.Metadata.Allowance,
			GasBudget:      req.Metadata.GasBudget,
		},
		req.UnlockConditions,
	)
	if nft != nil {
		out = NFTOutputFromBasicOutput(out.(*iotago.BasicOutput), nft)
	}
	return out
}

func outputMatchesSendAsAddress(output iotago.Output, outputID iotago.OutputID, address iotago.Address) bool {
	switch o := output.(type) {
	case *iotago.NFTOutput:
		if address.Equal(util.NFTIDFromNFTOutput(o, outputID).ToAddress()) {
			return true
		}
	case *iotago.AnchorOutput:
		if address.Equal(o.AnchorID.ToAddress()) {
			return true
		}
	}
	return false
}

func addNativeTokens(sumTokensOut iotago.NativeTokenSum, out iotago.Output) {
	if nt := out.FeatureSet().NativeToken(); nt != nil {
		s, ok := sumTokensOut[nt.ID]
		if !ok {
			s = new(big.Int)
		}
		s.Add(s, nt.Amount)
		sumTokensOut[nt.ID] = s
	}
}

func updateOutputsWhenSendingOnBehalfOf(
	senderKeyPair *cryptolib.KeyPair,
	senderAddress iotago.Address,
	unspentOutputs iotago.OutputSet,
	outputs iotago.TxEssenceOutputs,
	outputAssets *AssetsWithMana,
	creationSlot iotago.SlotIndex,
) (
	iotago.TxEssenceOutputs,
	*AssetsWithMana,
) {
	if senderAddress.Equal(senderKeyPair.Address()) {
		return outputs, outputAssets
	}
	// sending request "on behalf of" (need NFT or anchor output as input/output)

	for _, output := range outputs {
		if outputMatchesSendAsAddress(output, iotago.OutputID{}, senderAddress) {
			// if already present in the outputs, no need to do anything
			return outputs, outputAssets
		}
	}
	for outID, out := range unspentOutputs {
		// find the output that matches the "send as" address
		if !outputMatchesSendAsAddress(out, outID, senderAddress) {
			continue
		}
		// found the needed output
		outputs = append(outputs, out)
		assets, err := AssetsAndAvailableManaFromOutput(outID, out, creationSlot)
		if err != nil {
			panic(err)
		}
		outputAssets.Add(assets)
		return outputs, outputAssets
	}
	panic("unable to build tx, 'sendAs' output not found")
}
