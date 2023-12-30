// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package evmimpl

import (
	"math/big"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/evm/iscmagic"
)

// handler for ISCAccounts::getL2BalanceBaseTokens
func (h *magicContractHandler) GetL2BalanceBaseTokens(agentID iscmagic.ISCAgentID) uint64 {
	r := h.callView(accounts.ViewBalanceBaseToken.Message(agentID.MustUnwrap()))
	return uint64(lo.Must(accounts.ViewBalanceBaseToken.Output.Decode(r)))
}

// handler for ISCAccounts::getL2BalanceNativeTokens
func (h *magicContractHandler) GetL2BalanceNativeTokens(nativeTokenID iscmagic.NativeTokenID, agentID iscmagic.ISCAgentID) *big.Int {
	r := h.callView(accounts.ViewBalanceNativeToken.Message(agentID.MustUnwrap(), nativeTokenID.Unwrap()))
	return lo.Must(accounts.ViewBalanceNativeToken.Output.Decode(r))
}

// handler for ISCAccounts::getL2NFTs
func (h *magicContractHandler) GetL2NFTs(agentID iscmagic.ISCAgentID) []iscmagic.NFTID {
	r := h.callView(accounts.ViewAccountNFTs.Message(agentID.MustUnwrap()))
	return lo.Map(accounts.ViewAccountNFTs.Output.Decode(r), func(nftID iotago.NFTID, _ int) iscmagic.NFTID {
		return iscmagic.NFTID(nftID)
	})
}

// handler for ISCAccounts::getL2NFTAmount
func (h *magicContractHandler) GetL2NFTAmount(agentID iscmagic.ISCAgentID) *big.Int {
	r := h.callView(accounts.ViewAccountNFTAmount.Message(agentID.MustUnwrap()))
	n := lo.Must(accounts.ViewAccountNFTAmount.Output.Decode(r))
	return big.NewInt(int64(n))
}

// handler for ISCAccounts::getL2NFTsInCollection
func (h *magicContractHandler) GetL2NFTsInCollection(agentID iscmagic.ISCAgentID, collectionID iscmagic.NFTID) []iscmagic.NFTID {
	r := h.callView(accounts.ViewAccountNFTsInCollection.Message(agentID.MustUnwrap(), collectionID.Unwrap()))
	return lo.Map(accounts.ViewAccountNFTsInCollection.Output.Decode(r), func(nftID iotago.NFTID, _ int) iscmagic.NFTID {
		return iscmagic.NFTID(nftID)
	})
}

// handler for ISCAccounts::getL2NFTAmountInCollection
func (h *magicContractHandler) GetL2NFTAmountInCollection(agentID iscmagic.ISCAgentID, collectionID iscmagic.NFTID) *big.Int {
	r := h.callView(accounts.ViewAccountNFTAmountInCollection.Message(agentID.MustUnwrap(), collectionID.Unwrap()))
	n := lo.Must(accounts.ViewAccountNFTAmountInCollection.Output.Decode(r))
	return big.NewInt(int64(n))
}

// handler for ISCAccounts::foundryCreateNew
func (h *magicContractHandler) FoundryCreateNew(tokenScheme iotago.SimpleTokenScheme, allowance iscmagic.ISCAssets) uint32 {
	ret := h.call(
		accounts.FuncFoundryCreateNew.Message(&tokenScheme),
		allowance.Unwrap(),
	)
	return lo.Must(codec.Uint32.Decode(ret.Get(accounts.ParamFoundrySN)))
}

// handler for ISCAccounts::mintBaseTokens
func (h *magicContractHandler) MintNativeTokens(foundrySN uint32, amount *big.Int, allowance iscmagic.ISCAssets) {
	h.call(
		accounts.FuncFoundryModifySupply.MintTokens(foundrySN, amount),
		allowance.Unwrap(),
	)
}
