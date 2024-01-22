package accounts

import (
	"math/big"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/util"
)

// deprecated on v1.0.1-rc.16

func getBaseTokensDEPRECATED(state kv.KVStoreReader, accountKey kv.Key, _ *api.InfoResBaseToken) iotago.BaseToken {
	return iotago.BaseToken(lo.Must(codec.Uint64.Decode(state.Get(BaseTokensKey(accountKey)), 0)))
}

func setBaseTokensDEPRECATED(state kv.KVStore, accountKey kv.Key, amount iotago.BaseToken, _ *api.InfoResBaseToken) {
	state.Set(BaseTokensKey(accountKey), codec.Uint64.Encode(uint64(amount)))
}

const deprecatedBaseTokenDecimals = 6 // all deprecated state was saved with 6 decimals, so it's ok to hardcode this

func getBaseTokensFullDecimalsDEPRECATED(state kv.KVStoreReader, accountKey kv.Key) *big.Int {
	amount := lo.Must(codec.Uint64.Decode(state.Get(BaseTokensKey(accountKey)), 0))
	baseTokens, _ := util.BaseTokensDecimalsToEthereumDecimals(iotago.BaseToken(amount), deprecatedBaseTokenDecimals)
	return baseTokens
}

func setBaseTokensFullDecimalsDEPRECATED(state kv.KVStore, accountKey kv.Key, amount *big.Int) {
	baseTokens, _ := util.EthereumDecimalsToBaseTokenDecimals(amount, deprecatedBaseTokenDecimals)
	state.Set(BaseTokensKey(accountKey), codec.Uint64.Encode(uint64(baseTokens)))
}
