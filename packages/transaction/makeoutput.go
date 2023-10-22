package transaction

import (
	"fmt"
	"math/big"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/vm"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/util"
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
		par.Assets.WithMana(0),
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
	assets *isc.AssetsWithMana,
	metadata *isc.RequestMetadata,
	unlockConditions []iotago.UnlockCondition,
) *iotago.BasicOutput {
	if assets == nil {
		assets = isc.NewEmptyAssetsWithMana()
	}
	out := &iotago.BasicOutput{
		Amount: assets.BaseTokens,
		Conditions: iotago.BasicOutputUnlockConditions{
			&iotago.AddressUnlockCondition{Address: targetAddress},
		},
		Mana: assets.Mana,
	}
	if len(assets.NativeTokens) > 1 {
		panic("at most 1 native token is supported")
	}
	if len(assets.NativeTokens) > 0 && assets.NativeTokens[0].Amount.Cmp(util.Big0) > 0 {
		nt := assets.NativeTokens[0]
		out.Features = append(out.Features, nt.Clone())
	}

	if senderAddress != nil {
		out.Features = append(out.Features, &iotago.SenderFeature{
			Address: senderAddress,
		})
	}
	if metadata != nil {
		out.Features = append(out.Features, &iotago.MetadataFeature{
			Data: metadata.Bytes(),
		})
	}
	for _, c := range unlockConditions {
		out.Conditions = append(out.Conditions, c)
	}
	return out
}

func NFTOutputFromPostData(
	senderAddress iotago.Address,
	senderContract isc.ContractIdentity,
	par isc.RequestParameters,
	nft *isc.NFT,
) *iotago.NFTOutput {
	if len(par.Assets.NFTs) != 1 || nft.ID != par.Assets.NFTs[0] {
		panic("inconsistency: len(par.Assets.NFTs) != 1 || nft.ID != par.Assets.NFTs[0]")
	}
	basicOutput := BasicOutputFromPostData(senderAddress, senderContract, par)
	out := NFTOutputFromBasicOutput(basicOutput, nft)

	if !par.AdjustToMinimumStorageDeposit {
		return out
	}
	storageDeposit, err := parameters.RentStructure().MinDeposit(out)
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
			&iotago.MetadataFeature{Data: nft.Metadata},
		},
	}
	for _, f := range o.Features {
		out.Features = append(out.Features, f)
	}
	for _, c := range o.Conditions {
		out.Conditions = append(out.Conditions, c)
	}
	return out
}

func AssetsFromOutput(o iotago.Output) *isc.Assets {
	assets := &isc.Assets{
		BaseTokens: o.BaseTokenAmount(),
	}
	if nt := o.FeatureSet().NativeToken(); nt != nil {
		assets.NativeTokens = append(assets.NativeTokens, &iotago.NativeTokenFeature{
			ID:     nt.ID,
			Amount: new(big.Int).Set(nt.Amount),
		})
	}
	if o, ok := o.(*iotago.NFTOutput); ok {
		assets.NFTs = append(assets.NFTs, o.NFTID)
	}
	return assets
}

func AssetsAndManaFromOutput(
	oID iotago.OutputID,
	o iotago.Output,
	slotIndex iotago.SlotIndex,
) (*isc.AssetsWithMana, error) {
	assets := AssetsFromOutput(o)
	mana, err := vm.TotalManaIn(
		parameters.L1API().ManaDecayProvider(),
		parameters.RentStructure(),
		slotIndex,
		vm.InputSet{oID: o},
	)
	if err != nil {
		return nil, err
	}
	return isc.NewAssetsWithMana(assets, mana), nil
}

func AssetsAndStoredManaFromOutput(o iotago.Output) *isc.AssetsWithMana {
	return AssetsFromOutput(o).WithMana(o.StoredMana())
}

func AdjustToMinimumStorageDeposit[T iotago.Output](out T) T {
	storageDeposit, err := parameters.RentStructure().MinDeposit(out)
	if err != nil {
		panic(err)
	}
	if out.BaseTokenAmount() >= storageDeposit {
		return out
	}
	switch out := iotago.Output(out).(type) {
	case *iotago.AccountOutput:
		out.Amount = storageDeposit
	case *iotago.BasicOutput:
		out.Amount = storageDeposit
	case *iotago.FoundryOutput:
		out.Amount = storageDeposit
	case *iotago.NFTOutput:
		out.Amount = storageDeposit
	default:
		panic(fmt.Sprintf("no handler for output type %T", out))
	}
	return out
}
