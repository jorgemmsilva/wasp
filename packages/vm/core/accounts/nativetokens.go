package accounts

import (
	"math/big"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
)

func nativeTokensMapKey(accountKey kv.Key) string {
	return prefixNativeTokens + string(accountKey)
}

func (s *StateReader) NativeTokensMap(accountKey kv.Key) *collections.ImmutableMap {
	return collections.NewMapReadOnly(s.state, nativeTokensMapKey(accountKey))
}

func (s *StateWriter) nativeTokensMap(accountKey kv.Key) *collections.Map {
	return collections.NewMap(s.state, nativeTokensMapKey(accountKey))
}

func (s *StateReader) getNativeTokenAmount(accountKey kv.Key, tokenID iotago.NativeTokenID) *big.Int {
	r := new(big.Int)
	b := s.NativeTokensMap(accountKey).GetAt(tokenID[:])
	if len(b) > 0 {
		r.SetBytes(b)
	}
	return r
}

func (s *StateWriter) setNativeTokenAmount(accountKey kv.Key, tokenID iotago.NativeTokenID, n *big.Int) {
	if n.Sign() == 0 {
		s.nativeTokensMap(accountKey).DelAt(tokenID[:])
	} else {
		s.nativeTokensMap(accountKey).SetAt(tokenID[:], codec.BigIntAbs.Encode(n))
	}
}

func (s *StateReader) GetNativeTokenBalance(agentID isc.AgentID, nativeTokenID iotago.NativeTokenID) *big.Int {
	return s.getNativeTokenAmount(s.accountKey(agentID), nativeTokenID)
}

func (s *StateReader) GetNativeTokenBalanceTotal(nativeTokenID iotago.NativeTokenID) *big.Int {
	return s.getNativeTokenAmount(L2TotalsAccount, nativeTokenID)
}

func (s *StateReader) getNativeTokens(accountKey kv.Key) iotago.NativeTokenSum {
	ret := iotago.NativeTokenSum{}
	s.NativeTokensMap(accountKey).Iterate(func(idBytes []byte, val []byte) bool {
		id := lo.Must(isc.NativeTokenIDFromBytes(idBytes))
		ret[id] = new(big.Int).SetBytes(val)
		return true
	})
	return ret
}

func (s *StateReader) GetNativeTokensTotal() iotago.NativeTokenSum {
	return s.getNativeTokens(L2TotalsAccount)
}

func (s *StateReader) GetNativeTokens(agentID isc.AgentID) iotago.NativeTokenSum {
	return s.getNativeTokens(s.accountKey(agentID))
}
