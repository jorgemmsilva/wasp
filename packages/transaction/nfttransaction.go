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
	unspentOutputIDs iotago.OutputIDs,
	creationSlot iotago.SlotIndex,
) (*iotago.SignedTransaction, error) {
	senderAddress := issuerKeyPair.Address()

	storageDeposit := iotago.BaseToken(0)
	var outputs iotago.TxEssenceOutputs

	var issuerAddress iotago.Address = senderAddress
	nftsOut := make(map[iotago.NFTID]bool)

	addOutput := func(out *iotago.NFTOutput) {
		d, err := parameters.RentStructure().MinDeposit(out)
		if err != nil {
			panic(err)
		}
		out.Amount = d
		storageDeposit += d

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

	inputIDs, remainder, err := ComputeInputsAndRemainder(senderAddress, storageDeposit, nil, nftsOut, unspentOutputs, unspentOutputIDs)
	if err != nil {
		return nil, err
	}
	outputs = append(outputs, remainder...)

	inputsCommitment := inputIDs.OrderedSet(unspentOutputs).MustCommitment(parameters.L1API())
	return CreateAndSignTx(
		issuerKeyPair,
		inputIDs.UTXOInputs(),
		inputsCommitment,
		outputs,
		creationSlot,
	)
}
