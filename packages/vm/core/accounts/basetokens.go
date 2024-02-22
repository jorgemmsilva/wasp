package accounts

import (
	"math/big"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/util"
)

func (s *StateReader) getBaseTokens(accountKey kv.Key) (tokens iotago.BaseToken, remainder *big.Int) {
	switch s.ctx.SchemaVersion() {
	case 0:
		return iotago.BaseToken(lo.Must(codec.Uint64.Decode(s.state.Get(BaseTokensKey(accountKey)), 0))), big.NewInt(0)
	default:
		amount := s.getBaseTokensFullDecimals(accountKey)
		// convert from 18 decimals, discard the remainder
		return util.EthereumDecimalsToBaseTokenDecimals(amount, s.ctx.TokenInfo().Decimals)
	}
}

const v0BaseTokenDecimals = 6 // all v0 state was saved with 6 decimals

func (s *StateReader) getBaseTokensFullDecimals(accountKey kv.Key) *big.Int {
	switch s.ctx.SchemaVersion() {
	case 0:
		baseTokens, _ := s.getBaseTokens(accountKey)
		return util.BaseTokensDecimalsToEthereumDecimals(iotago.BaseToken(baseTokens), v0BaseTokenDecimals)
	default:
		return lo.Must(codec.BigIntAbs.Decode(s.state.Get(BaseTokensKey(accountKey)), big.NewInt(0)))
	}
}

func (s *StateWriter) setBaseTokens(accountKey kv.Key, amount iotago.BaseToken) {
	switch s.ctx.SchemaVersion() {
	case 0:
		s.state.Set(BaseTokensKey(accountKey), codec.Uint64.Encode(uint64(amount)))
	default:
		fullDecimals := util.BaseTokensDecimalsToEthereumDecimals(amount, s.ctx.TokenInfo().Decimals)
		s.setBaseTokensFullDecimals(accountKey, fullDecimals)
	}
}

func (s *StateWriter) setBaseTokensFullDecimals(accountKey kv.Key, amount *big.Int) {
	switch s.ctx.SchemaVersion() {
	case 0:
		baseTokens := util.MustEthereumDecimalsToBaseTokenDecimalsExact(amount, v0BaseTokenDecimals)
		s.setBaseTokens(accountKey, baseTokens)
	default:
		s.state.Set(BaseTokensKey(accountKey), codec.BigIntAbs.Encode(amount))
	}
}

// -------------------------------------------------------------------------------

func BaseTokensKey(accountKey kv.Key) kv.Key {
	return prefixBaseTokens + accountKey
}

func setBaseTokensNEW(state kv.KVStore, accountKey kv.Key, amount iotago.BaseToken, baseTokentokenInfo *api.InfoResBaseToken) {
	// convert to 18 decimals
	amountConverted := util.BaseTokensDecimalsToEthereumDecimals(amount, baseTokentokenInfo.Decimals)
	state.Set(BaseTokensKey(accountKey), codec.BigIntAbs.Encode(amountConverted))
}

func (s *StateWriter) AdjustAccountBaseTokens(
	account isc.AgentID,
	adjustment int64,
) {
	switch {
	case adjustment > 0:
		s.CreditToAccount(account, isc.NewFungibleTokens(iotago.BaseToken(adjustment), nil))
	case adjustment < 0:
		s.DebitFromAccount(account, isc.NewFungibleTokens(iotago.BaseToken(-adjustment), nil))
	}
}

func (s *StateReader) GetBaseTokensBalance(agentID isc.AgentID) (bts iotago.BaseToken, remainder *big.Int) {
	return s.getBaseTokens(s.accountKey(agentID))
}

func (s *StateReader) GetBaseTokensBalanceFullDecimals(agentID isc.AgentID) *big.Int {
	return s.getBaseTokensFullDecimals(s.accountKey(agentID))
}

func (s *StateReader) GetBaseTokensBalanceDiscardExtraDecimals(agentID isc.AgentID) iotago.BaseToken {
	bts, _ := s.getBaseTokens(s.accountKey(agentID))
	return bts
}
