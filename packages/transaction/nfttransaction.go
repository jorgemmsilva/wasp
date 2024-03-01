package transaction

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/util"
)

func NewMintNFTsTransaction(
	nftIssuerKeyPair cryptolib.VariantKeyPair,
	collectionOutputID *iotago.OutputID,
	target iotago.Address,
	immutableMetadata []iotago.MetadataFeatureEntries,
	unspentOutputs iotago.OutputSet,
	creationSlot iotago.SlotIndex,
	l1APIProvider iotago.APIProvider,
	blockIssuance *api.IssuanceBlockHeaderResponse,
) (*iotago.Block, error) {
	senderAddress := nftIssuerKeyPair.Address()

	outputAssets := NewEmptyAssetsWithMana()
	var outputs []iotago.Output

	var issuerAddress iotago.Address = senderAddress
	nftsOut := make(map[iotago.NFTID]bool)

	l1API := l1APIProvider.APIForSlot(creationSlot)

	addOutput := func(out *iotago.NFTOutput) {
		d, err := l1API.StorageScoreStructure().MinDeposit(out)
		if err != nil {
			panic(err)
		}
		out.Amount = d
		outputAssets.BaseTokens += d
		if out.NFTID != iotago.EmptyNFTID() {
			outputAssets.AddNFTs(out.NFTID)
		}
		outputs = append(outputs, out)
	}

	if collectionOutputID != nil {
		collectionOutputID := *collectionOutputID
		collectionOutput := unspentOutputs[collectionOutputID].(*iotago.NFTOutput)
		collectionID := util.NFTIDFromNFTOutput(collectionOutput, collectionOutputID)
		issuerAddress = collectionID.ToAddress()
		nftsOut[collectionID] = true

		out := collectionOutput.Clone().(*iotago.NFTOutput)
		out.NFTID = collectionID
		addOutput(out)
	}

	for _, immutableMetadata := range immutableMetadata {
		addOutput(&iotago.NFTOutput{
			NFTID: iotago.NFTID{},
			UnlockConditions: iotago.NFTOutputUnlockConditions{
				&iotago.AddressUnlockCondition{Address: target},
			},
			ImmutableFeatures: iotago.NFTOutputImmFeatures{
				&iotago.IssuerFeature{Address: issuerAddress},
				&iotago.MetadataFeature{Entries: immutableMetadata},
			},
		})
	}

	inputs, remainder, blockIssuerAccountID, err := ComputeInputsAndRemainder(
		senderAddress,
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
		TxBuilderFromInputsAndOutputs(l1API, inputs, outputs, nftIssuerKeyPair),
		blockIssuance,
		len(outputs)-1, // store mana in the last output
		blockIssuerAccountID,
		nftIssuerKeyPair,
	)
}
