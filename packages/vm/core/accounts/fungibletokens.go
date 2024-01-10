package accounts

import (
	"fmt"
	"math/big"

	"github.com/samber/lo"

	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/util"
)

// CreditToAccount brings new funds to the on chain ledger
func CreditToAccount(
	state kv.KVStore,
	agentID isc.AgentID,
	fts *isc.FungibleTokens,
	chainID isc.ChainID,
	baseToken *api.InfoResBaseToken,
) {
	if fts.IsEmpty() {
		return
	}
	creditToAccount(state, accountKey(agentID, chainID), fts, baseToken)
	creditToAccount(state, L2TotalsAccount, fts, baseToken)
	touchAccount(state, agentID, chainID)
}

// creditToAccount adds assets to the internal account map
func creditToAccount(
	state kv.KVStore,
	accountKey kv.Key,
	fts *isc.FungibleTokens,
	baseToken *api.InfoResBaseToken,
) {
	if fts.IsEmpty() {
		return
	}

	if fts.BaseTokens > 0 {
		setBaseTokens(state, accountKey, getBaseTokens(state, accountKey, baseToken)+fts.BaseTokens, baseToken)
	}
	for id, amount := range fts.NativeTokens {
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

func CreditToAccountFullDecimals(state kv.KVStore, agentID isc.AgentID, amount *big.Int, chainID isc.ChainID) {
	if !util.IsPositiveBigInt(amount) {
		return
	}
	creditToAccountFullDecimals(state, accountKey(agentID, chainID), amount)
	creditToAccountFullDecimals(state, L2TotalsAccount, amount)
	touchAccount(state, agentID, chainID)
}

// creditToAccountFullDecimals adds assets to the internal account map
// NOTE: this function does not take NFTs into account
func creditToAccountFullDecimals(state kv.KVStore, accountKey kv.Key, amount *big.Int) {
	setBaseTokensFullDecimals(state, accountKey, new(big.Int).Add(getBaseTokensFullDecimals(state, accountKey), amount))
}

// DebitFromAccount takes out assets balance the on chain ledger. If not enough it panics
func DebitFromAccount(
	state kv.KVStore,
	agentID isc.AgentID,
	fts *isc.FungibleTokens,
	chainID isc.ChainID,
	baseToken *api.InfoResBaseToken,
) {
	if fts.IsEmpty() {
		return
	}
	if !debitFromAccount(state, accountKey(agentID, chainID), fts, baseToken) {
		panic(fmt.Errorf("cannot debit (%s) from %s: %w", fts, agentID, ErrNotEnoughFunds))
	}
	if !debitFromAccount(state, L2TotalsAccount, fts, baseToken) {
		panic("debitFromAccount: inconsistent ledger state")
	}
	touchAccount(state, agentID, chainID)
}

// debitFromAccount debits assets from the internal accounts map
func debitFromAccount(state kv.KVStore, accountKey kv.Key, debit *isc.FungibleTokens, baseToken *api.InfoResBaseToken) bool {
	if debit.IsEmpty() {
		return true
	}

	// first check, then mutate
	balance := isc.NewEmptyFungibleTokens()
	if debit.BaseTokens > 0 {
		baseTokens := getBaseTokens(state, accountKey, baseToken)
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
		setBaseTokens(state, accountKey, balance.BaseTokens-debit.BaseTokens, baseToken)
	}
	for id, amount := range debit.NativeTokens {
		setNativeTokenAmount(state, accountKey, id, new(big.Int).Sub(balance.NativeTokens.ValueOrBigInt0(id), amount))
	}
	return true
}

// DebitFromAccountFullDecimals removes the amount from the chain ledger. If not enough it panics
func DebitFromAccountFullDecimals(state kv.KVStore, agentID isc.AgentID, amount *big.Int, chainID isc.ChainID) {
	if !util.IsPositiveBigInt(amount) {
		return
	}
	if !debitFromAccountFullDecimals(state, accountKey(agentID, chainID), amount) {
		panic(fmt.Errorf("cannot debit (%s) from %s: %w", amount.String(), agentID, ErrNotEnoughFunds))
	}

	if !debitFromAccountFullDecimals(state, L2TotalsAccount, amount) {
		panic("debitFromAccount: inconsistent ledger state")
	}
	touchAccount(state, agentID, chainID)
}

// debitFromAccountFullDecimals debits the amount from the internal accounts map
func debitFromAccountFullDecimals(state kv.KVStore, accountKey kv.Key, amount *big.Int) bool {
	balance := getBaseTokensFullDecimals(state, accountKey)
	if balance.Cmp(amount) < 0 {
		return false
	}
	setBaseTokensFullDecimals(state, accountKey, new(big.Int).Sub(balance, amount))
	return true
}

// getFungibleTokens returns the fungible tokens owned by an account (base tokens extra decimals will be discarded)
func getFungibleTokens(state kv.KVStoreReader, accountKey kv.Key, baseToken *api.InfoResBaseToken) *isc.FungibleTokens {
	ret := isc.NewEmptyFungibleTokens()
	ret.AddBaseTokens(getBaseTokens(state, accountKey, baseToken))
	nativeTokensMapR(state, accountKey).Iterate(func(idBytes []byte, val []byte) bool {
		ret.AddNativeTokens(
			lo.Must(isc.NativeTokenIDFromBytes(idBytes)),
			new(big.Int).SetBytes(val),
		)
		return true
	})
	return ret
}

func calcL2TotalFungibleTokens(state kv.KVStoreReader, baseToken *api.InfoResBaseToken) *isc.FungibleTokens {
	ret := isc.NewEmptyFungibleTokens()
	totalBaseTokens := big.NewInt(0)

	allAccountsMapR(state).IterateKeys(func(accountKey []byte) bool {
		// add all native tokens owned by each account
		nativeTokensMapR(state, kv.Key(accountKey)).Iterate(func(idBytes []byte, val []byte) bool {
			ret.AddNativeTokens(
				lo.Must(codec.NativeTokenID.Decode(idBytes)),
				new(big.Int).SetBytes(val),
			)
			return true
		})
		// use the full decimals for each account, so no dust balance is lost in the calculation
		baseTokensFullDecimals := getBaseTokensFullDecimals(state, kv.Key(accountKey))
		totalBaseTokens = new(big.Int).Add(totalBaseTokens, baseTokensFullDecimals)
		return true
	})

	// convert from 18 decimals, remainder must be 0
	ret.BaseTokens = util.MustEthereumDecimalsToBaseTokenDecimalsExact(totalBaseTokens, baseToken.Decimals)
	return ret
}

// GetAccountFungibleTokens returns all fungible tokens belonging to the agentID on the state
func GetAccountFungibleTokens(
	state kv.KVStoreReader,
	agentID isc.AgentID,
	chainID isc.ChainID,
	baseToken *api.InfoResBaseToken,
) *isc.FungibleTokens {
	return getFungibleTokens(state, accountKey(agentID, chainID), baseToken)
}

func GetTotalL2FungibleTokens(state kv.KVStoreReader, baseToken *api.InfoResBaseToken) *isc.FungibleTokens {
	return getFungibleTokens(state, L2TotalsAccount, baseToken)
}
