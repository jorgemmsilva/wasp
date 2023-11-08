package transaction

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/util"
)

func NewMintNFTsTransaction(
	issuerKeyPair *cryptolib.KeyPair,
	collectionOutputID *iotago.OutputID,
	target iotago.Address,
	immutableMetadata [][]byte,
	unspentOutputs iotago.OutputSet,
	creationSlot iotago.SlotIndex,
) (*iotago.SignedTransaction, error) {
	senderAddress := issuerKeyPair.Address()

	outputAssets := NewEmptyAssetsWithMana()
	var outputs iotago.TxEssenceOutputs

	var issuerAddress iotago.Address = senderAddress
	nftsOut := make(map[iotago.NFTID]bool)

	addOutput := func(out *iotago.NFTOutput) {
		d, err := parameters.Storage().MinDeposit(out)
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
			Conditions: iotago.NFTOutputUnlockConditions{
				&iotago.AddressUnlockCondition{Address: target},
			},
			ImmutableFeatures: iotago.NFTOutputImmFeatures{
				&iotago.IssuerFeature{Address: issuerAddress},
				&iotago.MetadataFeature{Data: immutableMetadata},
			},
		})
	}

	inputIDs, remainder, err := ComputeInputsAndRemainder(
		senderAddress,
		unspentOutputs,
		outputAssets,
		creationSlot,
	)
	if err != nil {
		return nil, err
	}
	outputs = append(outputs, remainder...)

	return CreateAndSignTx(
		issuerKeyPair,
		inputIDs.UTXOInputs(),
		outputs,
		creationSlot,
	)
}
