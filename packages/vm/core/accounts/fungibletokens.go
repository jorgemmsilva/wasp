package accounts

import (
	"fmt"
	"math/big"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/util"
)

// CreditToAccount brings new funds to the on chain ledger
func CreditToAccount(state kv.KVStore, agentID isc.AgentID, fts *isc.FungibleTokens, chainID isc.ChainID) {
	if fts.IsEmpty() {
		return
	}
	creditToAccount(state, accountKey(agentID, chainID), fts)
	creditToAccount(state, l2TotalsAccount, fts)
	touchAccount(state, agentID, chainID)
}

// creditToAccount adds assets to the internal account map
// NOTE: this function does not take NFTs into account
func creditToAccount(state kv.KVStore, accountKey kv.Key, assets *isc.FungibleTokens) {
	if assets.IsEmpty() {
		return
	}

	if assets.BaseTokens > 0 {
		setBaseTokens(state, accountKey, getBaseTokens(state, accountKey)+assets.BaseTokens)
	}
	for id, amount := range assets.NativeTokens {
		if amount.Sign() == 0 {
			continue
		}
		if amount.Sign() < 0 {
			panic(ErrBadAmount)
		}
		balance := getNativeTokenAmount(state, accountKey, id)
		balance.Add(balance, amount)
		if balance.Cmp(util.MaxUint256) > 0 {
			panic(ErrOverflow)
		}
		setNativeTokenAmount(state, accountKey, id, balance)
	}
}

// DebitFromAccount takes out assets balance the on chain ledger. If not enough it panics
func DebitFromAccount(state kv.KVStore, agentID isc.AgentID, fts *isc.FungibleTokens, chainID isc.ChainID) {
	if fts.IsEmpty() {
		return
	}
	if !debitFromAccount(state, accountKey(agentID, chainID), fts) {
		panic(fmt.Errorf("cannot debit (%s) from %s: %w", fts, agentID, ErrNotEnoughFunds))
	}
	if !debitFromAccount(state, l2TotalsAccount, fts) {
		panic("debitFromAccount: inconsistent ledger state")
	}
	touchAccount(state, agentID, chainID)
}

// debitFromAccount debits assets from the internal accounts map
func debitFromAccount(state kv.KVStore, accountKey kv.Key, debit *isc.FungibleTokens) bool {
	if debit.IsEmpty() {
		return true
	}

	// first check, then mutate
	balance := isc.NewEmptyFungibleTokens()
	if debit.BaseTokens > 0 {
		baseTokens := getBaseTokens(state, accountKey)
		if debit.BaseTokens > baseTokens {
			return false
		}
		balance.BaseTokens = baseTokens
	}
	for id, amount := range debit.NativeTokens {
		if amount.Sign() == 0 {
			continue
		}
		if amount.Sign() < 0 {
			panic(ErrBadAmount)
		}
		ntBalance := getNativeTokenAmount(state, accountKey, id)
		if ntBalance.Cmp(amount) < 0 {
			return false
		}
		balance.AddNativeTokens(id, ntBalance)
	}

	if debit.BaseTokens > 0 {
		setBaseTokens(state, accountKey, balance.BaseTokens-debit.BaseTokens)
	}
	for id, amount := range debit.NativeTokens {
		setNativeTokenAmount(state, accountKey, id, new(big.Int).Sub(balance.NativeTokens.ValueOrBigInt0(id), amount))
	}
	return true
}

func getFungibleTokens(state kv.KVStoreReader, accountKey kv.Key) *isc.FungibleTokens {
	ret := isc.NewEmptyFungibleTokens()
	ret.AddBaseTokens(getBaseTokens(state, accountKey))
	nativeTokensMapR(state, accountKey).Iterate(func(idBytes []byte, val []byte) bool {
		ret.AddNativeTokens(
			isc.MustNativeTokenIDFromBytes(idBytes),
			new(big.Int).SetBytes(val),
		)
		return true
	})
	return ret
}

func calcL2TotalFungibleTokens(state kv.KVStoreReader) *isc.FungibleTokens {
	ret := isc.NewEmptyFungibleTokens()
	allAccountsMapR(state).IterateKeys(func(key []byte) bool {
		ret.Add(getFungibleTokens(state, kv.Key(key)))
		return true
	})
	return ret
}

// GetAccountFungibleTokens returns all fungible tokens belonging to the agentID on the state
func GetAccountFungibleTokens(state kv.KVStoreReader, agentID isc.AgentID, chainID isc.ChainID) *isc.FungibleTokens {
	return getFungibleTokens(state, accountKey(agentID, chainID))
}

func GetTotalL2FungibleTokens(state kv.KVStoreReader) *isc.FungibleTokens {
	return getFungibleTokens(state, l2TotalsAccount)
}

func getAccountBalanceDict(state kv.KVStoreReader, accountKey kv.Key) dict.Dict {
	return getFungibleTokens(state, accountKey).ToDict()
}
