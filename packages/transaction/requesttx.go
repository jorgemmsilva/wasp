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
	fungibleTokens *isc.Assets,
	senderAddress iotago.Address,
	senderKeyPair *cryptolib.KeyPair,
	targetAddress iotago.Address,
	unspentOutputs iotago.OutputSet,
	unspentOutputIDs iotago.OutputIDs,
	unlockConditions []iotago.UnlockCondition,
	creationSlot iotago.SlotIndex,
	disableAutoAdjustStorageDeposit bool, // if true, the minimal storage deposit won't be adjusted automatically
) (*iotago.SignedTransaction, error) {
	output := MakeBasicOutput(
		targetAddress,
		senderAddress,
		fungibleTokens,
		nil,
		unlockConditions,
	)
	if !disableAutoAdjustStorageDeposit {
		output = AdjustToMinimumStorageDeposit(output)
	}

	storageDeposit, err := parameters.L1API().RentStructure().MinDeposit(output)
	if err != nil {
		return nil, err
	}
	if output.BaseTokenAmount() < storageDeposit {
		return nil, fmt.Errorf("%v: available %d < required %d base tokens",
			ErrNotEnoughBaseTokensForStorageDeposit, output.BaseTokenAmount(), storageDeposit)
	}

	sumBaseTokensOut := output.BaseTokenAmount()
	sumTokensOut := make(iotago.NativeTokenSum)
	addNativeTokens(sumTokensOut, output)

	tokenMap := iotago.NativeTokenSum{}
	for _, nativeToken := range fungibleTokens.NativeTokens {
		tokenMap[nativeToken.ID] = nativeToken.Amount
	}

	inputIDs, remainder, err := ComputeInputsAndRemainder(senderAddress,
		sumBaseTokensOut,
		sumTokensOut,
		map[iotago.NFTID]bool{},
		unspentOutputs,
		unspentOutputIDs,
	)
	if err != nil {
		return nil, err
	}

	outputs := append(iotago.TxEssenceOutputs{output}, remainder...)

	inputsCommitment := inputIDs.OrderedSet(unspentOutputs).MustCommitment(parameters.L1API())

	return CreateAndSignTx(
		senderKeyPair,
		inputIDs.UTXOInputs(),
		inputsCommitment,
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
	senderAddress iotago.Address, // might be different from the senderKP address (when sending as NFT or alias)
	unspentOutputs iotago.OutputSet,
	unspentOutputIDs iotago.OutputIDs,
	request *isc.RequestParameters,
	nft *isc.NFT,
	creationSlot iotago.SlotIndex,
	disableAutoAdjustStorageDeposit bool, // if true, the minimal storage deposit won't be adjusted automatically
) (*iotago.SignedTransaction, error) {
	outputs := iotago.TxEssenceOutputs{}
	sumBaseTokensOut := iotago.BaseToken(0)
	sumTokensOut := make(iotago.NativeTokenSum)
	sumNFTsOut := make(map[iotago.NFTID]bool)

	out := MakeRequestTransactionOutput(senderAddress, request, nft)
	if !disableAutoAdjustStorageDeposit {
		out = AdjustToMinimumStorageDeposit(out)
	}

	storageDeposit, err := parameters.L1API().RentStructure().MinDeposit(out)
	if err != nil {
		return nil, err
	}
	if out.BaseTokenAmount() < storageDeposit {
		return nil, fmt.Errorf("%v: available %d < required %d base tokens",
			ErrNotEnoughBaseTokensForStorageDeposit, out.BaseTokenAmount(), storageDeposit)
	}
	outputs = append(outputs, out)
	sumBaseTokensOut += out.BaseTokenAmount()
	addNativeTokens(sumTokensOut, out)
	if nft != nil {
		sumNFTsOut[nft.ID] = true
	}

	outputs, sumBaseTokensOut = updateOutputsWhenSendingOnBehalfOf(
		senderKeyPair,
		senderAddress,
		unspentOutputs,
		outputs,
		sumBaseTokensOut,
		sumTokensOut,
		sumNFTsOut,
	)

	inputIDs, remainder, err := ComputeInputsAndRemainder(senderKeyPair.Address(), sumBaseTokensOut, sumTokensOut, sumNFTsOut, unspentOutputs, unspentOutputIDs)
	if err != nil {
		return nil, err
	}

	outputs = append(outputs, remainder...)

	inputsCommitment := inputIDs.OrderedSet(unspentOutputs).MustCommitment(parameters.L1API())
	return CreateAndSignTx(
		senderKeyPair,
		inputIDs.UTXOInputs(),
		inputsCommitment,
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
		assets,
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
	case *iotago.AccountOutput:
		if address.Equal(o.AccountID.ToAddress()) {
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
	sumBaseTokensOut iotago.BaseToken,
	sumTokensOut iotago.NativeTokenSum,
	sumNFTsOut map[iotago.NFTID]bool,
) (
	iotago.TxEssenceOutputs,
	iotago.BaseToken,
) {
	if senderAddress.Equal(senderKeyPair.Address()) {
		return outputs, sumBaseTokensOut
	}
	// sending request "on behalf of" (need NFT or alias output as input/output)

	for _, output := range outputs {
		if outputMatchesSendAsAddress(output, iotago.OutputID{}, senderAddress) {
			// if already present in the outputs, no need to do anything
			return outputs, sumBaseTokensOut
		}
	}
	for outID, out := range unspentOutputs {
		// find the output that matches the "send as" address
		if !outputMatchesSendAsAddress(out, outID, senderAddress) {
			continue
		}
		if nftOut, ok := out.(*iotago.NFTOutput); ok {
			if nftOut.NFTID.Empty() {
				// if this is the first time the NFT output transitions, we need to fill the correct NFTID
				nftOut.NFTID = iotago.NFTIDFromOutputID(outID)
			}
			sumNFTsOut[nftOut.NFTID] = true
		}
		// found the needed output
		outputs = append(outputs, out)
		sumBaseTokensOut += out.BaseTokenAmount()
		addNativeTokens(sumTokensOut, out)
		return outputs, sumBaseTokensOut
	}
	panic("unable to build tx, 'sendAs' output not found")
}
