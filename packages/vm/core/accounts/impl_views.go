package accounts

import (
	"math/big"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/vm/core/errors/coreerrors"
)

// viewBalance returns the balances of the account belonging to the AgentID
func viewBalance(ctx isc.SandboxView, agentIDOpt *isc.AgentID) *isc.FungibleTokens {
	ctx.Log().Debugf("accounts.viewBalance")
	agentID := coreutil.FromOptional(agentIDOpt, ctx.Caller())
	return getFungibleTokens(ctx.StateR(), accountKey(agentID, ctx.ChainID()), ctx.TokenInfo())
}

// viewBalanceBaseToken returns the base tokens balance of the account belonging to the AgentID
func viewBalanceBaseToken(ctx isc.SandboxView, agentIDOpt *isc.AgentID) iotago.BaseToken {
	agentID := coreutil.FromOptional(agentIDOpt, ctx.Caller())
	return getBaseTokens(ctx.StateR(), accountKey(agentID, ctx.ChainID()), ctx.TokenInfo())
}

// viewBalanceBaseTokenEVM returns the base tokens balance of the account belonging to the AgentID (in the EVM format with 18 decimals)
// Params:
// - ParamAgentID (optional -- default: caller)
func viewBalanceBaseTokenEVM(ctx isc.SandboxView, agentIDOpt *isc.AgentID) *big.Int {
	agentID := coreutil.FromOptional(agentIDOpt, ctx.Caller())
	return getBaseTokensFullDecimals(ctx.StateR(), accountKey(agentID, ctx.ChainID()))
}

// viewBalanceNativeToken returns the native token balance of the account belonging to the AgentID
func viewBalanceNativeToken(ctx isc.SandboxView, agentIDOpt *isc.AgentID, ntID iotago.NativeTokenID) *big.Int {
	agentID := coreutil.FromOptional(agentIDOpt, ctx.Caller())
	return getNativeTokenAmount(ctx.StateR(), accountKey(agentID, ctx.ChainID()), ntID)
}

// viewTotalAssets returns total balances controlled by the chain
func viewTotalAssets(ctx isc.SandboxView) *isc.FungibleTokens {
	ctx.Log().Debugf("accounts.viewTotalAssets")
	return getFungibleTokens(ctx.StateR(), L2TotalsAccount, ctx.TokenInfo())
}

// viewAccounts returns list of all accounts
func viewAccounts(ctx isc.SandboxView) dict.Dict {
	return allAccountsAsDict(ctx.StateR())
}

// nonces are only sent with off-ledger requests
func viewGetAccountNonce(ctx isc.SandboxView, agentIDOpt *isc.AgentID) uint64 {
	agentID := coreutil.FromOptional(agentIDOpt, ctx.Caller())
	return AccountNonce(ctx.StateR(), agentID, ctx.ChainID())
}

// viewGetNativeTokenIDRegistry returns all native token ID accounted in the chain
func viewGetNativeTokenIDRegistry(ctx isc.SandboxView) []iotago.NativeTokenID {
	ret := make([]iotago.NativeTokenID, 0)
	nativeTokenOutputMapR(ctx.StateR()).IterateKeys(func(tokenID []byte) bool {
		ret = append(ret, lo.Must(codec.NativeTokenID.Decode(tokenID)))
		return true
	})
	return ret
}

// viewAccountFoundries returns the foundries owned by the given agentID
func viewAccountFoundries(ctx isc.SandboxView, accountOpt *isc.AgentID) map[uint32]struct{} {
	ret := make(map[uint32]struct{})
	account := coreutil.FromOptional(accountOpt, ctx.Caller())
	accountFoundriesMapR(ctx.StateR(), account).IterateKeys(func(foundry []byte) bool {
		ret[lo.Must(codec.Uint32.Decode(foundry))] = struct{}{}
		return true
	})
	return ret
}

var errFoundryNotFound = coreerrors.Register("foundry not found").Create()

// viewFoundryOutput takes serial number and returns corresponding foundry output in serialized form
func viewFoundryOutput(ctx isc.SandboxView, sn uint32) iotago.TxEssenceOutput {
	ctx.Log().Debugf("accounts.viewFoundryOutput")

	accountID, ok := ctx.ChainAccountID()
	ctx.Requiref(ok, "chain AccountID unknown")
	out, _ := GetFoundryOutput(ctx.StateR(), sn, accountID)
	if out == nil {
		panic(errFoundryNotFound)
	}
	return out
}

// viewAccountNFTs returns the NFTIDs of NFTs owned by an account
func viewAccountNFTs(ctx isc.SandboxView, agentIDOpt *isc.AgentID) []iotago.NFTID {
	ctx.Log().Debugf("accounts.viewAccountNFTs")
	agentID := coreutil.FromOptional(agentIDOpt, ctx.Caller())
	return getAccountNFTs(ctx.StateR(), agentID)
}

func viewAccountNFTAmount(ctx isc.SandboxView, agentIDOpt *isc.AgentID) uint32 {
	agentID := coreutil.FromOptional(agentIDOpt, ctx.Caller())
	return accountToNFTsMapR(ctx.StateR(), agentID).Len()
}

func viewAccountNFTsInCollection(ctx isc.SandboxView, agentIDOpt *isc.AgentID, collectionID iotago.NFTID) []iotago.NFTID {
	agentID := coreutil.FromOptional(agentIDOpt, ctx.Caller())
	return getAccountNFTsInCollection(ctx.StateR(), agentID, collectionID)
}

func viewAccountNFTAmountInCollection(ctx isc.SandboxView, agentIDOpt *isc.AgentID, collectionID iotago.NFTID) uint32 {
	agentID := coreutil.FromOptional(agentIDOpt, ctx.Caller())
	return nftsByCollectionMapR(ctx.StateR(), agentID, kv.Key(collectionID[:])).Len()
}

// viewNFTData returns the NFT data for a given NFTID
func viewNFTData(ctx isc.SandboxView, nftID iotago.NFTID) *isc.NFT {
	ctx.Log().Debugf("accounts.viewNFTData")
	nft := GetNFTData(ctx.StateR(), nftID)
	if nft == nil {
		panic("NFTID not found")
	}
	return nft
}
