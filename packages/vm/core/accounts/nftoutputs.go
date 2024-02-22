package accounts

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/kv/collections"
)

func (s *StateWriter) newNFTsArray() *collections.Array {
	return collections.NewArray(s.state, keyNewNFTs)
}

func (s *StateWriter) NFTOutputMap() *collections.Map {
	return collections.NewMap(s.state, keyNFTOutputRecords)
}

func (s *StateReader) nftOutputMap() *collections.ImmutableMap {
	return collections.NewMapReadOnly(s.state, keyNFTOutputRecords)
}

func (s *StateWriter) SaveNFTOutput(out *iotago.NFTOutput, outputIndex uint16) {
	tokenRec := NFTOutputRec{
		// TransactionID is unknown yet, will be filled next block
		OutputID: iotago.OutputIDFromTransactionIDAndIndex(iotago.TransactionID{}, outputIndex),
		Output:   out,
	}
	s.NFTOutputMap().SetAt(out.NFTID[:], tokenRec.Bytes())
	s.newNFTsArray().Push(out.NFTID[:])
}

func (s *StateWriter) updateNFTOutputIDs(anchorTxID iotago.TransactionID) {
	newNFTs := s.newNFTsArray()
	allNFTs := s.NFTOutputMap()
	n := newNFTs.Len()
	for i := uint32(0); i < n; i++ {
		nftID := newNFTs.GetAt(i)
		rec := mustNFTOutputRecFromBytes(allNFTs.GetAt(nftID))
		rec.OutputID = iotago.OutputIDFromTransactionIDAndIndex(anchorTxID, rec.OutputID.Index())
		allNFTs.SetAt(nftID, rec.Bytes())
	}
	newNFTs.Erase()
}

func (s *StateWriter) DeleteNFTOutput(nftID iotago.NFTID) {
	s.NFTOutputMap().DelAt(nftID[:])
}

func (s *StateReader) GetNFTOutput(nftID iotago.NFTID) (*iotago.NFTOutput, iotago.OutputID) {
	data := s.nftOutputMap().GetAt(nftID[:])
	if data == nil {
		return nil, iotago.OutputID{}
	}
	tokenRec := mustNFTOutputRecFromBytes(data)
	return tokenRec.Output, tokenRec.OutputID
}
