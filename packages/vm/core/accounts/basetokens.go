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

func BaseTokensKey(accountKey kv.Key) kv.Key {
	return prefixBaseTokens + accountKey
}

func getBaseTokensFullDecimals(state kv.KVStoreReader, accountKey kv.Key) *big.Int {
	return lo.Must(codec.BigIntAbs.Decode(state.Get(BaseTokensKey(accountKey)), big.NewInt(0)))
}

func setBaseTokensFullDecimals(state kv.KVStore, accountKey kv.Key, n *big.Int) {
	state.Set(BaseTokensKey(accountKey), codec.BigIntAbs.Encode(n))
}

func getBaseTokens(state kv.KVStoreReader, accountKey kv.Key, baseToken *api.InfoResBaseToken) iotago.BaseToken {
	amount := getBaseTokensFullDecimals(state, accountKey)
	// convert from 18 decimals, discard the remainder
	convertedAmount, _ := util.EthereumDecimalsToBaseTokenDecimals(amount, baseToken.Decimals)
	return convertedAmount
}

func setBaseTokens(state kv.KVStore, accountKey kv.Key, n iotago.BaseToken, baseToken *api.InfoResBaseToken) {
	// convert to 18 decimals
	amount := util.MustBaseTokensDecimalsToEthereumDecimalsExact(n, baseToken.Decimals)
	state.Set(BaseTokensKey(accountKey), codec.BigIntAbs.Encode(amount))
}

func AdjustAccountBaseTokens(
	state kv.KVStore,
	account isc.AgentID,
	adjustment int64,
	chainID isc.ChainID,
	baseToken *api.InfoResBaseToken,
) {
	switch {
	case adjustment > 0:
		CreditToAccount(state, account, isc.NewFungibleTokens(iotago.BaseToken(adjustment), nil), chainID, baseToken)
	case adjustment < 0:
		DebitFromAccount(state, account, isc.NewFungibleTokens(iotago.BaseToken(-adjustment), nil), chainID, baseToken)
	}
}

func GetBaseTokensBalance(state kv.KVStoreReader, agentID isc.AgentID, chainID isc.ChainID, baseToken *api.InfoResBaseToken) iotago.BaseToken {
	return getBaseTokens(state, accountKey(agentID, chainID), baseToken)
}
