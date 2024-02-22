package accounts

import (
	iotago "github.com/iotaledger/iota.go/v4"

	"github.com/iotaledger/wasp/packages/kv/collections"
)

func (s *StateWriter) newNativeTokensArray() *collections.Array {
	return collections.NewArray(s.state, keyNewNativeTokens)
}

func (s *StateWriter) nativeTokenOutputMap() *collections.Map {
	return collections.NewMap(s.state, keyNativeTokenOutputMap)
}

func (s *StateReader) nativeTokenOutputMap() *collections.ImmutableMap {
	return collections.NewMapReadOnly(s.state, keyNativeTokenOutputMap)
}

// SaveNativeTokenOutput map nativeTokenID -> foundryRec
func (s *StateWriter) SaveNativeTokenOutput(out *iotago.BasicOutput, outputIndex uint16) {
	tokenRec := nativeTokenOutputRec{
		// TransactionID is unknown yet, will be filled next block
		OutputID:          iotago.OutputIDFromTransactionIDAndIndex(iotago.TransactionID{}, outputIndex),
		StorageBaseTokens: out.Amount,
		Amount:            out.FeatureSet().NativeToken().Amount,
	}
	s.nativeTokenOutputMap().SetAt(out.FeatureSet().NativeToken().ID[:], tokenRec.Bytes())
	s.newNativeTokensArray().Push(out.FeatureSet().NativeToken().ID[:])
}

func (s *StateWriter) updateNativeTokenOutputIDs(anchorTxID iotago.TransactionID) {
	newNativeTokens := s.newNativeTokensArray()
	allNativeTokens := s.nativeTokenOutputMap()
	n := newNativeTokens.Len()
	for i := uint32(0); i < n; i++ {
		k := newNativeTokens.GetAt(i)
		rec := mustNativeTokenOutputRecFromBytes(allNativeTokens.GetAt(k))
		rec.OutputID = iotago.OutputIDFromTransactionIDAndIndex(anchorTxID, rec.OutputID.Index())
		allNativeTokens.SetAt(k, rec.Bytes())
	}
	newNativeTokens.Erase()
}

func (s *StateWriter) DeleteNativeTokenOutput(nativeTokenID iotago.NativeTokenID) {
	s.nativeTokenOutputMap().DelAt(nativeTokenID[:])
}

func (s *StateReader) GetNativeTokenOutput(nativeTokenID iotago.NativeTokenID) (*iotago.BasicOutput, iotago.OutputID) {
	data := s.nativeTokenOutputMap().GetAt(nativeTokenID[:])
	if data == nil {
		return nil, iotago.OutputID{}
	}
	tokenRec := mustNativeTokenOutputRecFromBytes(data)
	ret := &iotago.BasicOutput{
		Amount: tokenRec.StorageBaseTokens,
		UnlockConditions: iotago.BasicOutputUnlockConditions{
			&iotago.AddressUnlockCondition{Address: s.ctx.ChainID().AsAddress()},
		},
		Features: iotago.BasicOutputFeatures{
			&iotago.SenderFeature{
				Address: s.ctx.ChainID().AsAddress(),
			},
			&iotago.NativeTokenFeature{
				ID:     nativeTokenID,
				Amount: tokenRec.Amount,
			},
		},
	}
	return ret, tokenRec.OutputID
}
