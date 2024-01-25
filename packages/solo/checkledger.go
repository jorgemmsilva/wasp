package solo

import (
	"fmt"
	"math/big"

	"github.com/samber/lo"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/subrealm"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
)

// only used in internal tests and solo
func CheckLedger(v isc.SchemaVersion, store kv.KVStoreReader, checkpoint string) {
	state := subrealm.NewReadOnly(store, kv.Key(accounts.Contract.Hname().Bytes()))
	t := accounts.GetTotalL2FungibleTokens(v, state, testutil.TokenInfo)
	c := calcL2TotalFungibleTokens(v, state)
	if !t.Equals(c) {
		c := calcL2TotalFungibleTokens(v, state)
		panic(fmt.Sprintf("inconsistent on-chain account ledger @ checkpoint '%s'\n total assets: %s\ncalc total: %s\n",
			checkpoint, t, c))
	}
}

func calcL2TotalFungibleTokens(v isc.SchemaVersion, state kv.KVStoreReader) *isc.FungibleTokens {
	ret := isc.NewEmptyFungibleTokens()
	totalBaseTokens := big.NewInt(0)

	accounts.AllAccountsMapR(state).IterateKeys(func(accountKey []byte) bool {
		// add all native tokens owned by each account
		accounts.NativeTokensMapR(state, kv.Key(accountKey)).Iterate(func(idBytes []byte, val []byte) bool {
			ret.AddNativeTokens(
				lo.Must(isc.NativeTokenIDFromBytes(idBytes)),
				lo.Must(codec.BigIntAbs.Decode(val)),
			)
			return true
		})
		// use the full decimals for each account, so no dust balance is lost in the calculation
		baseTokensFullDecimals := accounts.GetBaseTokensFullDecimals(v)(state, kv.Key(accountKey))
		totalBaseTokens = new(big.Int).Add(totalBaseTokens, baseTokensFullDecimals)
		return true
	})

	// convert from 18 decimals, remainder must be 0
	ret.BaseTokens = util.MustEthereumDecimalsToBaseTokenDecimalsExact(totalBaseTokens, testutil.TokenInfo.Decimals)
	return ret
}
