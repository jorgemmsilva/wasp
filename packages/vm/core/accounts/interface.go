package accounts

import (
	"math"
	"math/big"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
)

var Contract = coreutil.NewContract(coreutil.CoreContractAccounts)

var (
	// Funcs
	FuncDeposit          = coreutil.NewEP0(Contract, "deposit")
	FuncFoundryCreateNew = coreutil.NewEP1(Contract, "foundryCreateNew",
		coreutil.FieldWithCodecOptional(ParamTokenScheme, codec.TokenScheme),
	)
	FuncFoundryDestroy = coreutil.NewEP1(Contract, "foundryDestroy",
		coreutil.FieldWithCodec(ParamFoundrySN, codec.Uint32),
	)
	FuncFoundryModifySupply    = EPFoundryModifySupply{EntryPointInfo: Contract.Func("foundryModifySupply")}
	FuncMintNFT                = EPMintNFT{EntryPointInfo: Contract.Func("mintNFT")}
	FuncTransferAccountToChain = coreutil.NewEP1(Contract, "transferAccountToChain",
		coreutil.FieldWithCodecOptional(ParamGasReserve, codec.Uint64),
	)
	FuncTransferAllowanceTo = coreutil.NewEP1(Contract, "transferAllowanceTo",
		coreutil.FieldWithCodec(ParamAgentID, codec.AgentID),
	)
	FuncWithdraw = coreutil.NewEP0(Contract, "withdraw")
	// TODO implement grant/claim protocol of moving ownership of the foundry
	//  Including ownership of the foundry by the common account/chain owner

	// Views
	ViewAccountFoundries = coreutil.NewViewEP11(Contract, "accountFoundries",
		coreutil.FieldWithCodecOptional(ParamAgentID, codec.AgentID),
		OutputSerialNumberSet{},
	)
	ViewAccountNFTAmount = coreutil.NewViewEP11(Contract, "accountNFTAmount",
		coreutil.FieldWithCodecOptional(ParamAgentID, codec.AgentID),
		coreutil.FieldWithCodec(ParamNFTAmount, codec.Uint32),
	)
	ViewAccountNFTAmountInCollection = coreutil.NewViewEP21(Contract, "accountNFTAmountInCollection",
		coreutil.FieldWithCodecOptional(ParamAgentID, codec.AgentID),
		coreutil.FieldWithCodec(ParamCollectionID, codec.NFTID),
		coreutil.FieldWithCodec(ParamNFTAmount, codec.Uint32),
	)
	ViewAccountNFTs = coreutil.NewViewEP11(Contract, "accountNFTs",
		coreutil.FieldWithCodecOptional(ParamAgentID, codec.AgentID),
		OutputNFTIDs{},
	)
	ViewAccountNFTsInCollection = coreutil.NewViewEP21(Contract, "accountNFTsInCollection",
		coreutil.FieldWithCodecOptional(ParamAgentID, codec.AgentID),
		coreutil.FieldWithCodec(ParamCollectionID, codec.NFTID),
		OutputNFTIDs{},
	)
	ViewNFTIDbyMintID = coreutil.NewViewEP11(Contract, "NFTIDbyMintID",
		coreutil.FieldWithCodec(ParamMintID, codec.Bytes),
		coreutil.FieldWithCodec(ParamNFTID, codec.NFTID),
	)
	ViewAccounts = coreutil.NewViewEP01(Contract, "accounts",
		OutputAccountList{},
	)
	ViewBalance = coreutil.NewViewEP11(Contract, "balance",
		coreutil.FieldWithCodecOptional(ParamAgentID, codec.AgentID),
		OutputFungibleTokens{},
	)
	ViewBalanceBaseToken = coreutil.NewViewEP11(Contract, "balanceBaseToken",
		coreutil.FieldWithCodecOptional(ParamAgentID, codec.AgentID),
		coreutil.FieldWithCodec(ParamBalance, codec.BaseToken),
	)
	ViewBalanceNativeToken = coreutil.NewViewEP21(Contract, "balanceNativeToken",
		coreutil.FieldWithCodecOptional(ParamAgentID, codec.AgentID),
		coreutil.FieldWithCodec(ParamNativeTokenID, codec.NativeTokenID),
		coreutil.FieldWithCodec(ParamBalance, codec.BigIntAbs),
	)
	ViewFoundryOutput = coreutil.NewViewEP11(Contract, "foundryOutput",
		coreutil.FieldWithCodec(ParamFoundrySN, codec.Uint32),
		coreutil.FieldWithCodec(ParamFoundryOutputBin, codec.Output),
	)
	ViewGetAccountNonce = coreutil.NewViewEP11(Contract, "getAccountNonce",
		coreutil.FieldWithCodecOptional(ParamAgentID, codec.AgentID),
		coreutil.FieldWithCodec(ParamAccountNonce, codec.Uint64),
	)
	ViewGetNativeTokenIDRegistry = coreutil.NewViewEP01(Contract, "getNativeTokenIDRegistry",
		OutputNativeTokenIDs{},
	)
	ViewNFTData = coreutil.NewViewEP11(Contract, "nftData",
		coreutil.FieldWithCodec(ParamNFTID, codec.NFTID),
		coreutil.FieldWithCodec(ParamNFTData, codec.NewCodecEx(isc.NFTFromBytes)),
	)
	ViewTotalAssets = coreutil.NewViewEP01(Contract, "totalAssets",
		OutputFungibleTokens{},
	)
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

func (e EPFoundryModifySupply) MintTokens(foundrySN uint32, delta *big.Int) isc.Message {
	return e.EntryPointInfo.Message(dict.Dict{
		ParamFoundrySN:      codec.Uint32.Encode(foundrySN),
		ParamSupplyDeltaAbs: codec.BigIntAbs.Encode(delta),
	})
}

func (e EPFoundryModifySupply) DestroyTokens(foundrySN uint32, delta *big.Int) isc.Message {
	return e.MintTokens(foundrySN, delta).
		WithParam(ParamDestroyTokens, codec.Bool.Encode(true))
}

func (e EPFoundryModifySupply) WithHandler(f func(isc.Sandbox, uint32, *big.Int, bool) dict.Dict) *coreutil.EntryPointHandler[isc.Sandbox] {
	return e.EntryPointInfo.WithHandler(func(ctx isc.Sandbox) dict.Dict {
		d := ctx.Params().Dict
		sn := lo.Must(codec.Uint32.Decode(d[ParamFoundrySN]))
		delta := lo.Must(codec.BigIntAbs.Decode(d[ParamSupplyDeltaAbs]))
		destroy := lo.Must(codec.Bool.Decode(d[ParamDestroyTokens], false))
		return f(ctx, sn, delta, destroy)
	})
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

func (e EPMintNFT) WithHandler(f func(isc.Sandbox, []byte, isc.AgentID, bool, iotago.NFTID) dict.Dict) *coreutil.EntryPointHandler[isc.Sandbox] {
	return e.EntryPointInfo.WithHandler(func(ctx isc.Sandbox) dict.Dict {
		d := ctx.Params().Dict
		immutableMetadata := lo.Must(codec.Bytes.Decode(d[ParamNFTImmutableData]))
		target := lo.Must(codec.AgentID.Decode(d[ParamAgentID]))
		withdraw := lo.Must(codec.Bool.Decode(d[ParamNFTWithdrawOnMint], false))
		collID := lo.Must(codec.NFTID.Decode(d[ParamCollectionID], iotago.NFTID{}))
		return f(ctx, immutableMetadata, target, withdraw, collID)
	})
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

type OutputNFTIDs struct{}

func (_ OutputNFTIDs) Encode(nftIDs []iotago.NFTID) dict.Dict {
	// TODO: add pagination?
	if len(nftIDs) > math.MaxUint16 {
		panic("too many NFTs")
	}
	return codec.SliceToArray(codec.NFTID, nftIDs, ParamNFTIDs)
}

func (_ OutputNFTIDs) Decode(r dict.Dict) ([]iotago.NFTID, error) {
	return codec.SliceFromArray(codec.NFTID, r, ParamNFTIDs)
}

type OutputSerialNumberSet struct{}

func (_ OutputSerialNumberSet) Encode(sns map[uint32]struct{}) dict.Dict {
	return codec.SliceToDictKeys(codec.Uint32, lo.Keys(sns))
}

func (_ OutputSerialNumberSet) Has(r dict.Dict, sn uint32) bool {
	return r.Has(kv.Key(codec.Uint32.Encode(sn)))
}

func (_ OutputSerialNumberSet) Decode(r dict.Dict) (map[uint32]struct{}, error) {
	sns, err := codec.SliceFromDictKeys(codec.Uint32, r)
	if err != nil {
		return nil, err
	}
	return lo.SliceToMap(sns, func(sn uint32) (uint32, struct{}) { return sn, struct{}{} }), nil
}

type OutputNativeTokenIDs struct{}

func (_ OutputNativeTokenIDs) Encode(ids []iotago.NativeTokenID) dict.Dict {
	return codec.SliceToDictKeys(codec.NativeTokenID, ids)
}

func (_ OutputNativeTokenIDs) Decode(r dict.Dict) ([]iotago.NativeTokenID, error) {
	return codec.SliceFromDictKeys(codec.NativeTokenID, r)
}

type OutputFungibleTokens struct{}

func (_ OutputFungibleTokens) Encode(fts *isc.FungibleTokens) dict.Dict {
	return fts.ToDict()
}

func (_ OutputFungibleTokens) Decode(r dict.Dict) (*isc.FungibleTokens, error) {
	return isc.FungibleTokensFromDict(r)
}

type OutputAccountList struct{ coreutil.RawDictCodec }

func (_ OutputAccountList) DecodeAccounts(allAccounts dict.Dict, chainID isc.ChainID) ([]isc.AgentID, error) {
	return codec.SliceFromDictKeys(
		codec.NewCodecEx(func(b []byte) (isc.AgentID, error) { return agentIDFromKey(kv.Key(b), chainID) }),
		allAccounts,
	)
}
