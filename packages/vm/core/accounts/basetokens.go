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

type (
	getBaseTokensFn             func(state kv.KVStoreReader, accountKey kv.Key, baseTokentokenInfo *api.InfoResBaseToken) iotago.BaseToken
	setBaseTokensFn             func(state kv.KVStore, accountKey kv.Key, amount iotago.BaseToken, baseTokentokenInfo *api.InfoResBaseToken)
	GetBaseTokensFullDecimalsFn func(state kv.KVStoreReader, accountKey kv.Key) *big.Int
	setBaseTokensFullDecimalsFn func(state kv.KVStore, accountKey kv.Key, amount *big.Int)
)

func getBaseTokens(v isc.SchemaVersion) getBaseTokensFn {
	switch v {
	case 0:
		return getBaseTokensDEPRECATED
	default:
		return getBaseTokensNEW
	}
}

func setBaseTokens(v isc.SchemaVersion) setBaseTokensFn {
	switch v {
	case 0:
		return setBaseTokensDEPRECATED
	default:
		return setBaseTokensNEW
	}
}

func GetBaseTokensFullDecimals(v isc.SchemaVersion) GetBaseTokensFullDecimalsFn {
	switch v {
	case 0:
		return getBaseTokensFullDecimalsDEPRECATED
	default:
		return getBaseTokensFullDecimalsNEW
	}
}

func setBaseTokensFullDecimals(v isc.SchemaVersion) setBaseTokensFullDecimalsFn {
	switch v {
	case 0:
		return setBaseTokensFullDecimalsDEPRECATED
	default:
		return setBaseTokensFullDecimalsNEW
	}
}

// -------------------------------------------------------------------------------

func BaseTokensKey(accountKey kv.Key) kv.Key {
	return prefixBaseTokens + accountKey
}

func getBaseTokensFullDecimalsNEW(state kv.KVStoreReader, accountKey kv.Key) *big.Int {
	return lo.Must(codec.BigIntAbs.Decode(state.Get(BaseTokensKey(accountKey)), big.NewInt(0)))
}

func setBaseTokensFullDecimalsNEW(state kv.KVStore, accountKey kv.Key, amount *big.Int) {
	state.Set(BaseTokensKey(accountKey), codec.BigIntAbs.Encode(amount))
}

func getBaseTokensNEW(state kv.KVStoreReader, accountKey kv.Key, baseTokentokenInfo *api.InfoResBaseToken) iotago.BaseToken {
	amount := getBaseTokensFullDecimalsNEW(state, accountKey)
	// convert from 18 decimals, discard the remainder
	convertedAmount, _ := util.EthereumDecimalsToBaseTokenDecimals(amount, baseTokentokenInfo.Decimals)
	return convertedAmount
}

func setBaseTokensNEW(state kv.KVStore, accountKey kv.Key, amount iotago.BaseToken, baseTokentokenInfo *api.InfoResBaseToken) {
	// convert to 18 decimals
	amountConverted := util.MustBaseTokensDecimalsToEthereumDecimalsExact(amount, baseTokentokenInfo.Decimals)
	state.Set(BaseTokensKey(accountKey), codec.BigIntAbs.Encode(amountConverted))
}

func AdjustAccountBaseTokens(
	v isc.SchemaVersion,
	state kv.KVStore,
	account isc.AgentID,
	adjustment int64,
	chainID isc.ChainID,
	baseToken *api.InfoResBaseToken,
	hrp iotago.NetworkPrefix,
) {
	switch {
	case adjustment > 0:
		CreditToAccount(v, state, account, isc.NewFungibleTokens(iotago.BaseToken(adjustment), nil), chainID, baseToken)
	case adjustment < 0:
		DebitFromAccount(v, state, account, isc.NewFungibleTokens(iotago.BaseToken(-adjustment), nil), chainID, baseToken, hrp)
	}
}

func GetBaseTokensBalance(v isc.SchemaVersion, state kv.KVStoreReader, agentID isc.AgentID, chainID isc.ChainID, baseToken *api.InfoResBaseToken) iotago.BaseToken {
	return getBaseTokens(v)(state, accountKey(agentID, chainID), baseToken)
}
