package accounts

import (
	"math"

	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/util"
)

// viewBalance returns the balances of the account belonging to the AgentID
// Params:
// - ParamAgentID
func viewBalance(ctx isc.SandboxView) []byte {
	ctx.Log().Debugf("accounts.viewBalance")
	aid, err := ctx.Params().GetAgentID(ParamAgentID)
	ctx.RequireNoError(err)
	return getAccountBalanceDict(ctx.StateR(), accountKey(aid)).Bytes()
}

// viewBalanceBaseToken returns the base tokens balance of the account belonging to the AgentID
// Params:
// - ParamAgentID
// Returns: {ParamBalance: uint64}
func viewBalanceBaseToken(ctx isc.SandboxView) []byte {
	amount := getBaseTokens(ctx.StateR(), accountKey(ctx.Params().MustGetAgentID(ParamAgentID)))
	return util.MustSerialize(amount)
}

// viewBalanceNativeToken returns the native token balance of the account belonging to the AgentID
// Params:
// - ParamAgentID
// - ParamNativeTokenID
// Returns: {ParamBalance: big.Int}
func viewBalanceNativeToken(ctx isc.SandboxView) []byte {
	nativeTokenID := ctx.Params().MustGetNativeTokenID(ParamNativeTokenID)
	bal := getNativeTokenAmount(
		ctx.StateR(),
		accountKey(ctx.Params().MustGetAgentID(ParamAgentID)),
		nativeTokenID,
	)
	return util.MustSerialize(bal)
}

// viewTotalAssets returns total balances controlled by the chain
func viewTotalAssets(ctx isc.SandboxView) []byte {
	ctx.Log().Debugf("accounts.viewTotalAssets")
	return getAccountBalanceDict(ctx.StateR(), l2TotalsAccount).Bytes()
}

// viewAccounts returns list of all accounts
func viewAccounts(ctx isc.SandboxView) []byte {
	return allAccountsAsDict(ctx.StateR()).Bytes()
}

// nonces are only sent with off-ledger requests
func viewGetAccountNonce(ctx isc.SandboxView) []byte {
	account := ctx.Params().MustGetAgentID(ParamAgentID)
	nonce := GetMaxAssumedNonce(ctx.StateR(), account)
	return util.MustSerialize(nonce)
}

// viewGetNativeTokenIDRegistry returns all native token ID accounted in the chain
func viewGetNativeTokenIDRegistry(ctx isc.SandboxView) []byte {
	mapping := nativeTokenOutputMapR(ctx.StateR())
	ret := make([][]byte, mapping.MustLen())
	i := 0
	mapping.MustIterate(func(elemKey []byte, value []byte) bool {
		ret[i] = elemKey
		i++
		return true
	})
	return util.MustSerialize(ret)
}

// viewAccountFoundries returns the foundries owned by the given agentID
func viewAccountFoundries(ctx isc.SandboxView) []byte {
	account := ctx.Params().MustGetAgentID(ParamAgentID)
	foundries := accountFoundriesMapR(ctx.StateR(), account)
	ret := make([][]byte, foundries.MustLen())
	i := 0
	foundries.MustIterate(func(k []byte, v []byte) bool {
		ret[i] = k
		i++
		return true
	})
	return util.MustSerialize(ret)
}

// viewFoundryOutput takes serial number and returns corresponding foundry output in serialized form
func viewFoundryOutput(ctx isc.SandboxView) []byte {
	ctx.Log().Debugf("accounts.viewFoundryOutput")

	sn := ctx.Params().MustGetUint32(ParamFoundrySN)
	out, _, _ := GetFoundryOutput(ctx.StateR(), sn, ctx.ChainID())
	ctx.Requiref(out != nil, "foundry #%d does not exist", sn)
	outBin, err := out.Serialize(serializer.DeSeriModeNoValidation, nil)
	ctx.RequireNoError(err, "internal: error while serializing foundry output")
	return outBin
}

// viewAccountNFTs returns the NFTIDs of NFTs owned by an account
func viewAccountNFTs(ctx isc.SandboxView) []byte {
	ctx.Log().Debugf("accounts.viewAccountNFTs")
	aid := ctx.Params().MustGetAgentID(ParamAgentID)
	nftIDs := getAccountNFTs(ctx.StateR(), aid)

	if len(nftIDs) > math.MaxUint16 {
		panic("too many NFTs")
	}
	return util.MustSerialize(nftIDs)
}

func viewAccountNFTAmount(ctx isc.SandboxView) []byte {
	aid := ctx.Params().MustGetAgentID(ParamAgentID)
	ret := uint32(nftsMapR(ctx.StateR(), aid).MustLen())
	return util.MustSerialize(ret)
}

func viewAccountNFTsInCollection(ctx isc.SandboxView) []byte {
	aid := ctx.Params().MustGetAgentID(ParamAgentID)
	collectionID := codec.MustDecodeNFTID(ctx.Params().MustGet(ParamCollectionID))
	nftIDs := getAccountNFTsInCollection(ctx.StateR(), aid, collectionID)

	if len(nftIDs) > math.MaxUint16 {
		panic("too many NFTs")
	}
	return util.MustSerialize(nftIDs)
}

func viewAccountNFTAmountInCollection(ctx isc.SandboxView) []byte {
	aid := ctx.Params().MustGetAgentID(ParamAgentID)
	collectionID := codec.MustDecodeNFTID(ctx.Params().MustGet(ParamCollectionID))
	ret := uint32(nftsByCollectionMapR(ctx.StateR(), aid, kv.Key(collectionID[:])).MustLen())
	return util.MustSerialize(ret)
}

// viewNFTData returns the NFT data for a given NFTID
func viewNFTData(ctx isc.SandboxView) []byte {
	ctx.Log().Debugf("accounts.viewNFTData")
	nftID := codec.MustDecodeNFTID(ctx.Params().MustGetBytes(ParamNFTID))
	data := MustGetNFTData(ctx.StateR(), nftID)
	return data.Bytes()
}
