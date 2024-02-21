package accounts

import (
	"fmt"
	"math/big"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/util"
)

// CreditToAccount brings new funds to the on chain ledger
func (s *StateWriter) CreditToAccount(agentID isc.AgentID, fts *isc.FungibleTokens) {
	s.CreditToAccountFullDecimals(
		agentID,
		util.BaseTokensDecimalsToEthereumDecimals(fts.BaseTokens, s.ctx.TokenInfo().Decimals),
		fts.NativeTokens,
	)
}

func (s *StateWriter) CreditToAccountFullDecimals(
	agentID isc.AgentID,
	baseTokens *big.Int,
	nativeTokens iotago.NativeTokenSum,
) {
	if baseTokens.Sign() == 0 && len(nativeTokens) == 0 {
		return
	}
	s.creditToAccount(s.accountKey(agentID), baseTokens, nativeTokens)
	s.creditToAccount(L2TotalsAccount, baseTokens, nativeTokens)
	s.touchAccount(agentID)
}

// creditToAccount adds assets to the internal account map
func (s *StateWriter) creditToAccount(
	accountKey kv.Key,
	baseTokens *big.Int,
	nativeTokens iotago.NativeTokenSum,
) {
	if baseTokens.Sign() == 0 && len(nativeTokens) == 0 {
		return
	}

	if baseTokens.Sign() > 0 {
		s.setBaseTokensFullDecimals(accountKey, new(big.Int).Add(s.getBaseTokensFullDecimals(accountKey), baseTokens))
	}
	for id, amount := range nativeTokens {
		if amount.Sign() == 0 {
			continue
		}
		if amount.Sign() < 0 {
			panic(ErrBadAmount)
		}
		balance := s.getNativeTokenAmount(accountKey, id)
		balance.Add(balance, amount)
		if balance.Cmp(util.MaxUint256) > 0 {
			panic(ErrOverflow)
		}
		s.setNativeTokenAmount(accountKey, id, balance)
	}
}

// DebitFromAccount takes out assets balance the on chain ledger. If not enough it panics
func (s *StateWriter) DebitFromAccount(agentID isc.AgentID, fts *isc.FungibleTokens) {
	s.DebitFromAccountFullDecimals(
		agentID,
		util.BaseTokensDecimalsToEthereumDecimals(fts.BaseTokens, s.ctx.TokenInfo().Decimals),
		fts.NativeTokens,
	)
}

func (s *StateWriter) DebitFromAccountFullDecimals(
	agentID isc.AgentID,
	baseTokens *big.Int,
	nativeTokens iotago.NativeTokenSum,
) {
	if baseTokens.Sign() == 0 && len(nativeTokens) == 0 {
		return
	}
	if !s.debitFromAccount(s.accountKey(agentID), baseTokens, nativeTokens) {
		panic(fmt.Errorf("cannot debit from %s: %w", agentID.Bech32(s.ctx.L1API().ProtocolParameters().Bech32HRP()), ErrNotEnoughFunds))
	}
	if !s.debitFromAccount(L2TotalsAccount, baseTokens, nativeTokens) {
		panic("debitFromAccount: inconsistent ledger state")
	}
	s.touchAccount(agentID)
}

// debitFromAccount debits assets from the internal accounts map
func (s *StateWriter) debitFromAccount(
	accountKey kv.Key,
	debitBaseTokens *big.Int,
	debitNativeTokens iotago.NativeTokenSum,
) bool {
	if debitBaseTokens.Sign() == 0 && len(debitNativeTokens) == 0 {
		return true
	}

	// first check, then mutate
	var resultBaseTokens *big.Int
	resultNativeTokens := iotago.NativeTokenSum{}
	if debitBaseTokens.Sign() > 0 {
		balance := s.getBaseTokensFullDecimals(accountKey)
		if balance.Cmp(debitBaseTokens) < 0 {
			return false
		}
		resultBaseTokens = new(big.Int).Sub(balance, debitBaseTokens)
	}
	for id, debitAmount := range debitNativeTokens {
		if debitAmount.Sign() == 0 {
			continue
		}
		if debitAmount.Sign() < 0 {
			panic(ErrBadAmount)
		}
		ntBalance := s.getNativeTokenAmount(accountKey, id)
		if ntBalance.Cmp(debitAmount) < 0 {
			return false
		}
		resultNativeTokens[id] = new(big.Int).Sub(ntBalance, debitAmount)
	}

	if resultBaseTokens != nil {
		s.setBaseTokensFullDecimals(accountKey, resultBaseTokens)
	}
	for id, amount := range resultNativeTokens {
		s.setNativeTokenAmount(accountKey, id, amount)
	}
	return true
}

// getFungibleTokens returns the fungible tokens owned by an account (base tokens extra decimals will be discarded)
func (s *StateReader) getFungibleTokensDiscardExtraDecimals(accountKey kv.Key) *isc.FungibleTokens {
	ret := isc.NewEmptyFungibleTokens()
	bts, _ := s.getBaseTokens(accountKey)
	ret.AddBaseTokens(bts)
	s.NativeTokensMap(accountKey).Iterate(func(idBytes []byte, val []byte) bool {
		ret.AddNativeTokens(
			lo.Must(isc.NativeTokenIDFromBytes(idBytes)),
			new(big.Int).SetBytes(val),
		)
		return true
	})
	return ret
}

// GetAccountFungibleTokens returns all fungible tokens belonging to the agentID on the state (base tokens extra decimals will be discarded)
func (s *StateReader) GetAccountFungibleTokensDiscardExtraDecimals(agentID isc.AgentID) *isc.FungibleTokens {
	return s.getFungibleTokensDiscardExtraDecimals(s.accountKey(agentID))
}

func (s *StateReader) GetTotalL2FungibleTokens() *isc.FungibleTokens {
	fullDecimals := s.getBaseTokensFullDecimals(L2TotalsAccount)
	nts := s.getNativeTokens(L2TotalsAccount)
	return isc.NewFungibleTokens(
		util.MustEthereumDecimalsToBaseTokenDecimalsExact(fullDecimals, s.ctx.TokenInfo().Decimals),
		nts,
	)
}

func (s *StateReader) GetTotalL2FungibleTokensDiscardExtraDecimals() *isc.FungibleTokens {
	return s.getFungibleTokensDiscardExtraDecimals(L2TotalsAccount)
}
