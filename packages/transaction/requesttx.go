package transaction

import (
	"fmt"
	"math/big"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/util"
)

// NewTransferTransaction creates a basic output transaction that sends L1 Token to another L1 address
func NewTransferTransaction(
	ftokens *isc.FungibleTokens,
	mana iotago.Mana,
	senderAddress iotago.Address,
	senderKeyPair cryptolib.VariantKeyPair,
	targetAddress iotago.Address,
	unspentOutputs iotago.OutputSet,
	unlockConditions []iotago.UnlockCondition,
	creationSlot iotago.SlotIndex,
	disableAutoAdjustStorageDeposit bool, // if true, the minimal storage deposit won't be adjusted automatically
	l1APIProvider iotago.APIProvider,
	blockIssuance *api.IssuanceBlockHeaderResponse,
) (*iotago.Block, error) {
	l1API := l1APIProvider.APIForSlot(creationSlot)
	output := MakeBasicOutput(
		targetAddress,
		senderAddress,
		ftokens,
		mana,
		nil,
		unlockConditions,
	)
	if !disableAutoAdjustStorageDeposit {
		output = AdjustToMinimumStorageDeposit(output, l1API)
	}

	storageDeposit, err := l1API.StorageScoreStructure().MinDeposit(output)
	if err != nil {
		return nil, err
	}
	if output.BaseTokenAmount() < storageDeposit {
		return nil, fmt.Errorf("%v: available %d < required %d base tokens",
			ErrNotEnoughBaseTokensForStorageDeposit, output.BaseTokenAmount(), storageDeposit)
	}

	inputs, remainder, blockIssuerAccountID, err := ComputeInputsAndRemainder(
		senderAddress,
		unspentOutputs,
		NewAssetsWithMana(isc.FungibleTokensFromOutput(output).ToAssets(), mana),
		creationSlot,
		l1APIProvider,
	)
	if err != nil {
		return nil, err
	}

	outputs := append([]iotago.Output{output}, remainder...)

	return FinalizeTxAndBuildBlock(
		l1API,
		TxBuilderFromInputsAndOutputs(l1API, inputs, outputs, senderKeyPair),
		blockIssuance,
		len(outputs)-1, // store mana in the last output
		blockIssuerAccountID,
		senderKeyPair,
	)
}

// NewRequestTransaction creates a transaction including one or more requests to a chain.
// Empty assets in the request data defaults to 1 base token, which later is adjusted to the minimum storage deposit
// Assumes all unspentOutputs and corresponding unspentOutputIDs can be used as inputs, i.e. are
// unlockable for the sender address
func NewRequestTransaction(
	senderKeyPair cryptolib.VariantKeyPair,
	senderAddress iotago.Address, // might be different from the senderKP address (when sending as NFT or anchor)
	unspentOutputs iotago.OutputSet,
	request *isc.RequestParameters,
	nft *isc.NFT,
	creationSlot iotago.SlotIndex,
	disableAutoAdjustStorageDeposit bool, // if true, the minimal storage deposit won't be adjusted automatically
	l1APIProvider iotago.APIProvider,
	blockIssuance *api.IssuanceBlockHeaderResponse,
) (*iotago.Block, error) {
	outputs := []iotago.Output{}

	l1API := l1APIProvider.APIForSlot(creationSlot)

	out := MakeRequestTransactionOutput(senderAddress, request, nft)
	if !disableAutoAdjustStorageDeposit {
		out = AdjustToMinimumStorageDeposit(out, l1API)
	}

	storageDeposit, err := l1API.StorageScoreStructure().MinDeposit(out)
	if err != nil {
		return nil, err
	}
	if out.BaseTokenAmount() < storageDeposit {
		return nil, fmt.Errorf("%v: available %d < required %d base tokens",
			ErrNotEnoughBaseTokensForStorageDeposit, out.BaseTokenAmount(), storageDeposit)
	}
	outputs = append(outputs, out)
	outputAssets := NewAssetsWithMana(isc.AssetsFromOutput(out, iotago.OutputID{}), out.StoredMana())

	outputs, outputAssets = updateOutputsWhenSendingOnBehalfOf(
		senderKeyPair,
		senderAddress,
		unspentOutputs,
		outputs,
		outputAssets,
		creationSlot,
		l1APIProvider,
	)

	inputs, remainder, blockIssuerAccountID, err := ComputeInputsAndRemainder(
		senderKeyPair.Address(),
		unspentOutputs,
		outputAssets,
		creationSlot,
		l1APIProvider,
	)
	if err != nil {
		return nil, err
	}

	outputs = append(outputs, remainder...)

	return FinalizeTxAndBuildBlock(
		l1API,
		TxBuilderFromInputsAndOutputs(l1API, inputs, outputs, senderKeyPair),
		blockIssuance,
		len(outputs)-1, // store mana in the last output
		blockIssuerAccountID,
		senderKeyPair,
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
		&assets.FungibleTokens,
		0,
		&isc.RequestMetadata{
			SenderContract: isc.EmptyContractIdentity(),
			Message:        req.Metadata.Message,
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
	senderKeyPair cryptolib.VariantKeyPair,
	senderAddress iotago.Address,
	unspentOutputs iotago.OutputSet,
	outputs []iotago.Output,
	outputAssets *AssetsWithMana,
	creationSlot iotago.SlotIndex,
	l1APIProvider iotago.APIProvider,
) (
	[]iotago.Output,
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
		outputs = append(outputs, updateID(out, outID))
		assets, err := AssetsAndAvailableManaFromOutput(outID, out, creationSlot, l1APIProvider)
		if err != nil {
			panic(err)
		}
		outputAssets.Add(assets)
		return outputs, outputAssets
	}
	panic("unable to build tx, 'sendAs' output not found")
}

func updateID(out iotago.Output, outID iotago.OutputID) iotago.Output {
	if out, ok := out.(*iotago.NFTOutput); ok && out.NFTID == iotago.EmptyNFTID() {
		out = out.Clone().(*iotago.NFTOutput)
		out.NFTID = util.NFTIDFromNFTOutput(out, outID)
		return out
	}
	if out, ok := out.(*iotago.AnchorOutput); ok && out.AnchorID == iotago.EmptyAnchorID {
		out = out.Clone().(*iotago.AnchorOutput)
		out.AnchorID = util.AnchorIDFromAnchorOutput(out, outID)
		return out
	}
	if out, ok := out.(*iotago.AccountOutput); ok && out.AccountID == iotago.EmptyAccountID {
		out = out.Clone().(*iotago.AccountOutput)
		out.AccountID = util.AccountIDFromAccountOutput(out, outID)
		return out
	}
	return out
}
