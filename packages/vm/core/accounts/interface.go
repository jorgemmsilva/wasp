package accounts

import (
	"math/big"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/kv/dict"
)

var Contract = coreutil.NewContract(coreutil.CoreContractAccounts)

var (
	// Funcs
	FuncDeposit          = coreutil.NewEP0(Contract, "deposit")
	FuncFoundryCreateNew = coreutil.NewEP1(Contract, "foundryCreateNew",
		ParamTokenScheme, codec.TokenScheme,
	)
	FuncFoundryDestroy = coreutil.NewEP1(Contract, "foundryDestroy",
		ParamFoundrySN, codec.Uint32,
	)
	FuncFoundryModifySupply    = EPFoundryModifySupply{EntryPointInfo: Contract.Func("foundryModifySupply")}
	FuncMintNFT                = EPMintNFT{EntryPointInfo: Contract.Func("mintNFT")}
	FuncTransferAccountToChain = coreutil.NewEP1(Contract, "transferAccountToChain",
		ParamGasReserve, codec.Uint64,
	)
	FuncTransferAllowanceTo = coreutil.NewEP1(Contract, "transferAllowanceTo",
		ParamAgentID, codec.AgentID,
	)
	FuncWithdraw = coreutil.NewEP0(Contract, "withdraw")
	// TODO implement grant/claim protocol of moving ownership of the foundry
	//  Including ownership of the foundry by the common account/chain owner

	// Views
	ViewAccountFoundries = EPViewAccountFoundries{EP1: coreutil.NewViewEP1(Contract, "accountFoundries",
		ParamAgentID, codec.AgentID,
	)}
	ViewAccountNFTAmount = coreutil.NewViewEP11(Contract, "accountNFTAmount",
		ParamAgentID, codec.AgentID,
		ParamNFTAmount, codec.Uint32,
	)
	ViewAccountNFTAmountInCollection = coreutil.NewViewEP21(Contract, "accountNFTAmountInCollection",
		ParamAgentID, codec.AgentID,
		ParamCollectionID, codec.NFTID,
		ParamNFTAmount, codec.Uint32,
	)
	ViewAccountNFTs = EPViewAccountNFTs{EP1: coreutil.NewViewEP1(Contract, "accountNFTs",
		ParamAgentID, codec.AgentID,
	)}
	ViewAccountNFTsInCollection = EPViewAccountNFTsInCollection{EP2: coreutil.NewViewEP2(Contract, "accountNFTsInCollection",
		ParamAgentID, codec.AgentID,
		ParamCollectionID, codec.NFTID,
	)}
	ViewNFTIDbyMintID = coreutil.NewViewEP11(Contract, "NFTIDbyMintID",
		ParamMintID, codec.Bytes,
		ParamNFTID, codec.NFTID,
	)
	ViewAccounts = EPViewAccounts{EP0: coreutil.NewViewEP0(Contract, "accounts")}
	ViewBalance  = EPViewBalance{EP1: coreutil.NewViewEP1(Contract, "balance",
		ParamAgentID, codec.AgentID,
	)}
	ViewBalanceBaseToken = coreutil.NewViewEP11(Contract, "balanceBaseToken",
		ParamAgentID, codec.AgentID,
		ParamBalance, codec.BaseToken,
	)
	ViewBalanceNativeToken = coreutil.NewViewEP21(Contract, "balanceNativeToken",
		ParamAgentID, codec.AgentID,
		ParamNativeTokenID, codec.NativeTokenID,
		ParamBalance, codec.BigIntAbs,
	)
	ViewFoundryOutput = coreutil.NewViewEP11(Contract, "foundryOutput",
		ParamFoundrySN, codec.Uint32,
		ParamFoundryOutputBin, codec.Output,
	)
	ViewGetAccountNonce = coreutil.NewViewEP11(Contract, "getAccountNonce",
		ParamAgentID, codec.AgentID,
		ParamAccountNonce, codec.Uint64,
	)
	ViewGetNativeTokenIDRegistry = EPViewNativeTokenIDRegistry{EP0: coreutil.NewViewEP0(Contract, "getNativeTokenIDRegistry")}
	ViewNFTData                  = coreutil.NewViewEP11(Contract, "nftData",
		ParamNFTID, codec.NFTID,
		ParamNFTData, codec.NewCodecEx(isc.NFTFromBytes),
	)
	ViewTotalAssets = EPViewTotalAssets{EP0: coreutil.NewViewEP0(Contract, "totalAssets")}
)

// request parameters
const (
	ParamAccountNonce           = "n"
	ParamAgentID                = "a"
	ParamBalance                = "B"
	ParamCollectionID           = "C"
	ParamDestroyTokens          = "y"
	ParamForceMinimumBaseTokens = "f"
	ParamFoundryOutputBin       = "b"
	ParamFoundrySN              = "s"
	ParamGasReserve             = "g"
	ParamNFTAmount              = "A"
	ParamNFTData                = "e"
	ParamNFTID                  = "z"
	ParamNFTIDs                 = "i"
	ParamNFTImmutableData       = "I"
	ParamNFTWithdrawOnMint      = "w"
	ParamMintID                 = "D"
	ParamNativeTokenID          = "N"
	ParamSupplyDeltaAbs         = "d"
	ParamTokenScheme            = "t"
)

type EPFoundryModifySupply struct {
	coreutil.EntryPointInfo[isc.Sandbox]
}

func (f EPFoundryModifySupply) MintTokens(foundrySN uint32, delta *big.Int) isc.Message {
	return f.EntryPointInfo.Message(dict.Dict{
		ParamFoundrySN:      codec.Uint32.Encode(foundrySN),
		ParamSupplyDeltaAbs: codec.BigIntAbs.Encode(delta),
	})
}

func (f EPFoundryModifySupply) DestroyTokens(foundrySN uint32, delta *big.Int) isc.Message {
	return f.MintTokens(foundrySN, delta).
		WithParam(ParamDestroyTokens, codec.Bool.Encode(true))
}

type EPMintNFT struct {
	coreutil.EntryPointInfo[isc.Sandbox]
}

type EPMintNFTMessage struct{ isc.Message }

func (e EPMintNFT) Message(immutableMetadata []byte, target isc.AgentID) EPMintNFTMessage {
	return EPMintNFTMessage{Message: e.EntryPointInfo.Message(dict.Dict{
		ParamNFTImmutableData: immutableMetadata,
		ParamAgentID:          codec.AgentID.Encode(target),
	})}
}

func (e EPMintNFTMessage) WithdrawOnMint(v bool) EPMintNFTMessage {
	e.Params[ParamNFTWithdrawOnMint] = codec.Bool.Encode(v)
	return e
}

func (e EPMintNFTMessage) WithCollectionID(v iotago.NFTID) EPMintNFTMessage {
	e.Params[ParamCollectionID] = codec.NFTID.Encode(v)
	return e
}

func (e EPMintNFTMessage) Build() isc.Message {
	return e.Message
}

type EPViewAccountFoundries struct {
	coreutil.EP1[isc.SandboxView, isc.AgentID]
	Output FieldAccountFoundries
}

type FieldAccountFoundries struct{}

func (e FieldAccountFoundries) HasFoundry(r dict.Dict, sn uint32) bool {
	return r[kv.Key(codec.Uint32.Encode(sn))] != nil
}

func (e FieldAccountFoundries) Decode(r dict.Dict) (map[uint32]struct{}, error) {
	sns := map[uint32]struct{}{}
	for k := range r {
		sn, err := codec.Uint32.Decode([]byte(k))
		if err != nil {
			return nil, err
		}
		sns[sn] = struct{}{}
	}
	return sns, nil
}

type FieldNativeTokenIDs struct{}

func (e FieldNativeTokenIDs) Decode(r dict.Dict) []iotago.NativeTokenID {
	ids := collections.NewArrayReadOnly(r, ParamNFTIDs)
	ret := make([]iotago.NativeTokenID, ids.Len())
	for i := range ret {
		copy(ret[i][:], ids.GetAt(uint32(i)))
	}
	return ret
}

type FieldNFTIDs struct{}

func (e FieldNFTIDs) Decode(r dict.Dict) []iotago.NFTID {
	nftIDs := collections.NewArrayReadOnly(r, ParamNFTIDs)
	ret := make([]iotago.NFTID, nftIDs.Len())
	for i := range ret {
		copy(ret[i][:], nftIDs.GetAt(uint32(i)))
	}
	return ret
}

type FieldFungibleTokens struct{}

func (e FieldFungibleTokens) Decode(r dict.Dict) (*isc.FungibleTokens, error) {
	return isc.FungibleTokensFromDict(r)
}

type EPViewAccountNFTs struct {
	coreutil.EP1[isc.SandboxView, isc.AgentID]
	Output FieldNFTIDs
}

type EPViewAccountNFTsInCollection struct {
	coreutil.EP2[isc.SandboxView, isc.AgentID, iotago.NFTID]
	Output FieldNFTIDs
}

type EPViewAccounts struct {
	coreutil.EP0[isc.SandboxView]
	Output FieldAccountList
}

type FieldAccountList struct{}

func (e FieldAccountList) Decode(r dict.Dict, chainID isc.ChainID) ([]isc.AgentID, error) {
	keys := r.KeysSorted()
	ret := make([]isc.AgentID, 0, len(keys))
	for _, key := range keys {
		aid, err := agentIDFromKey(key, chainID)
		if err != nil {
			return nil, err
		}
		ret = append(ret, aid)
	}
	return ret, nil
}

type EPViewNativeTokenIDRegistry struct {
	coreutil.EP0[isc.SandboxView]
	Output FieldNativeTokenIDs
}

type EPViewBalance struct {
	coreutil.EP1[isc.SandboxView, isc.AgentID]
	Output FieldFungibleTokens
}

type EPViewTotalAssets struct {
	coreutil.EP0[isc.SandboxView]
	Output FieldFungibleTokens
}
