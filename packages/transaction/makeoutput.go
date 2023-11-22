package transaction

import (
	"math/big"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
)

// BasicOutputFromPostData creates extended output object from parameters.
// It automatically adjusts amount of base tokens required for the storage deposit
func BasicOutputFromPostData(
	senderAddress iotago.Address,
	senderContract isc.ContractIdentity,
	par isc.RequestParameters,
) *iotago.BasicOutput {
	metadata := par.Metadata
	if metadata == nil {
		// if metadata is not specified, target is nil. It corresponds to sending funds to the plain L1 address
		metadata = &isc.SendMetadata{}
	}

	ret := MakeBasicOutput(
		par.TargetAddress,
		senderAddress,
		&par.Assets.FungibleTokens,
		0,
		&isc.RequestMetadata{
			SenderContract: senderContract,
			TargetContract: metadata.TargetContract,
			EntryPoint:     metadata.EntryPoint,
			Params:         metadata.Params,
			Allowance:      metadata.Allowance,
			GasBudget:      metadata.GasBudget,
		},
		par.UnlockConditions,
	)
	if par.AdjustToMinimumStorageDeposit {
		return AdjustToMinimumStorageDeposit(ret)
	}
	return ret
}

// MakeBasicOutput creates new output from input parameters (ignoring storage deposit).
func MakeBasicOutput(
	targetAddress iotago.Address,
	senderAddress iotago.Address,
	ftokens *isc.FungibleTokens,
	mana iotago.Mana,
	metadata *isc.RequestMetadata,
	unlockConditions []iotago.UnlockCondition,
) *iotago.BasicOutput {
	out := &iotago.BasicOutput{
		Amount: ftokens.BaseTokens,
		UnlockConditions: iotago.BasicOutputUnlockConditions{
			&iotago.AddressUnlockCondition{Address: targetAddress},
		},
		Mana: mana,
	}
	if senderAddress != nil {
		out.Features = append(out.Features, &iotago.SenderFeature{
			Address: senderAddress,
		})
	}
	if metadata != nil {
		out.Features = append(out.Features, &iotago.MetadataFeature{
			Entries: iotago.MetadataFeatureEntries{"": metadata.Bytes()},
		})
	}
	if id, amount, ok := MustSingleNativeToken(ftokens); ok {
		out.Features = append(out.Features, &iotago.NativeTokenFeature{
			ID:     id,
			Amount: new(big.Int).Set(amount),
		})
	}
	for _, c := range unlockConditions {
		out.UnlockConditions = append(out.UnlockConditions, c)
	}
	return out
}

func NFTOutputFromPostData(
	senderAddress iotago.Address,
	senderContract isc.ContractIdentity,
	par isc.RequestParameters,
	nft *isc.NFT,
	l1API iotago.API,
) *iotago.NFTOutput {
	if len(par.Assets.NFTs) != 1 || nft.ID != par.Assets.NFTs[0] {
		panic("inconsistency: len(par.Assets.NFTs) != 1 || nft.ID != par.Assets.NFTs[0]")
	}
	basicOutput := BasicOutputFromPostData(senderAddress, senderContract, par)
	out := NFTOutputFromBasicOutput(basicOutput, nft)

	if !par.AdjustToMinimumStorageDeposit {
		return out
	}
	storageDeposit, err := l1API.StorageScoreStructure().MinDeposit(out)
	if err != nil {
		panic(err)
	}
	if out.Amount < storageDeposit {
		// adjust the amount to the minimum required
		out.Amount = storageDeposit
	}
	return out
}

func NFTOutputFromBasicOutput(o *iotago.BasicOutput, nft *isc.NFT) *iotago.NFTOutput {
	out := &iotago.NFTOutput{
		Amount: o.Amount,
		NFTID:  nft.ID,
		ImmutableFeatures: iotago.NFTOutputImmFeatures{
			&iotago.IssuerFeature{Address: nft.Issuer},
			&iotago.MetadataFeature{Entries: nft.Metadata},
		},
	}
	for _, f := range o.Features {
		out.Features = append(out.Features, f)
	}
	for _, c := range o.UnlockConditions {
		out.UnlockConditions = append(out.UnlockConditions, c)
	}
	return out
}
