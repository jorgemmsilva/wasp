// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package evm

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/vm/core/evm/evmnames"
)

var Contract = coreutil.NewContract(evmnames.Contract)

var (
	// FuncSendTransaction is the main entry point, called by an
	// evmOffLedgerTxRequest in order to process an Ethereum tx (e.g.
	// eth_sendRawTransaction).
	FuncSendTransaction = Contract.Func(evmnames.FuncSendTransaction)

	// FuncCallContract is the entry point called by an evmOffLedgerCallRequest
	// in order to process a view call or gas estimation (e.g. eth_call, eth_estimateGas).
	FuncCallContract = Contract.Func(evmnames.FuncCallContract)

	FuncRegisterERC20NativeToken              = EPRegisterERC20NativeToken{EntryPointInfo: Contract.Func(evmnames.FuncRegisterERC20NativeToken)}
	FuncRegisterERC20NativeTokenOnRemoteChain = EPRegisterERC20NativeTokenOnRemoteChain{EntryPointInfo: Contract.Func(evmnames.FuncRegisterERC20NativeTokenOnRemoteChain)}
	FuncRegisterERC20ExternalNativeToken      = EPRegisterERC20ExteralNativeToken{EntryPointInfo: Contract.Func(evmnames.FuncRegisterERC20ExternalNativeToken)}
	FuncRegisterERC721NFTCollection           = coreutil.NewEP1(Contract, evmnames.FuncRegisterERC721NFTCollection,
		FieldNFTCollectionID, codec.NFTID,
	)
	FuncNewL1Deposit = EPNewL1Deposit{EntryPointInfo: Contract.Func(evmnames.FuncNewL1Deposit)}

	ViewGetChainID = coreutil.NewViewEP01(Contract, evmnames.ViewGetChainID,
		FieldResult, codec.Uint16,
	)

	ViewGetERC20ExternalNativeTokenAddress = coreutil.NewViewEP11(Contract, evmnames.ViewGetERC20ExternalNativeTokenAddress,
		FieldNativeTokenID, codec.NativeTokenID,
		FieldResult, codec.EthereumAddress,
	)
)

const (
	FieldTransaction      = evmnames.FieldTransaction
	FieldCallMsg          = evmnames.FieldCallMsg
	FieldChainID          = evmnames.FieldChainID
	FieldAddress          = evmnames.FieldAddress
	FieldAssets           = evmnames.FieldAssets
	FieldAgentID          = evmnames.FieldAgentID
	FieldTransactionIndex = evmnames.FieldTransactionIndex
	FieldTransactionHash  = evmnames.FieldTransactionHash
	FieldResult           = evmnames.FieldResult
	FieldBlockNumber      = evmnames.FieldBlockNumber
	FieldBlockHash        = evmnames.FieldBlockHash
	FieldFilterQuery      = evmnames.FieldFilterQuery
	FieldBlockKeepAmount  = evmnames.FieldBlockKeepAmount // int32

	FieldNativeTokenID      = evmnames.FieldNativeTokenID
	FieldFoundrySN          = evmnames.FieldFoundrySN         // uint32
	FieldTokenName          = evmnames.FieldTokenName         // string
	FieldTokenTickerSymbol  = evmnames.FieldTokenTickerSymbol // string
	FieldTokenDecimals      = evmnames.FieldTokenDecimals     // uint8
	FieldNFTCollectionID    = evmnames.FieldNFTCollectionID   // NFTID
	FieldFoundryTokenScheme = evmnames.FieldFoundryTokenScheme
	FieldTargetAddress      = evmnames.FieldTargetAddress

	FieldAgentIDDepositOriginator = evmnames.FieldAgentIDDepositOriginator
)

const (
	// TODO shouldn't this be different between chain, to prevent replay attacks? (maybe derived from ISC ChainID)
	DefaultChainID = uint16(1074) // IOTA -- get it?
)

type ERC20NativeTokenParams struct {
	FoundrySN    uint32
	Name         string
	TickerSymbol string
	Decimals     uint8
}

func (e ERC20NativeTokenParams) ToDict() dict.Dict {
	return dict.Dict{
		FieldFoundrySN:         codec.Uint32.Encode(e.FoundrySN),
		FieldTokenName:         codec.String.Encode(e.Name),
		FieldTokenTickerSymbol: codec.String.Encode(e.TickerSymbol),
		FieldTokenDecimals:     codec.Uint8.Encode(e.Decimals),
	}
}

func ERC20NativeTokenParamsFromContext(p *isc.Params) ERC20NativeTokenParams {
	return ERC20NativeTokenParams{
		FoundrySN:    lo.Must(codec.Uint32.Decode(p.Get(FieldFoundrySN))),
		Name:         lo.Must(codec.String.Decode(p.Get(FieldTokenName))),
		TickerSymbol: lo.Must(codec.String.Decode(p.Get(FieldTokenTickerSymbol))),
		Decimals:     lo.Must(codec.Uint8.Decode(p.Get(FieldTokenDecimals))),
	}
}

type EPRegisterERC20NativeToken struct {
	coreutil.EntryPointInfo[isc.Sandbox]
}

func (e EPRegisterERC20NativeToken) Message(token ERC20NativeTokenParams) isc.Message {
	return e.EntryPointInfo.Message(token.ToDict())
}

type EPRegisterERC20NativeTokenOnRemoteChain struct {
	coreutil.EntryPointInfo[isc.Sandbox]
}

func (e EPRegisterERC20NativeTokenOnRemoteChain) Message(token ERC20NativeTokenParams, targetChain iotago.Address) isc.Message {
	params := token.ToDict()
	params[FieldTargetAddress] = codec.Address.Encode(targetChain)
	return e.EntryPointInfo.Message(params)
}

type EPRegisterERC20ExteralNativeToken struct {
	coreutil.EntryPointInfo[isc.Sandbox]
}

func (e EPRegisterERC20ExteralNativeToken) Message(token ERC20NativeTokenParams, foundryTokenScheme iotago.TokenScheme, accountAddress iotago.Address) isc.Message {
	params := token.ToDict()
	params[FieldFoundryTokenScheme] = codec.TokenScheme.Encode(foundryTokenScheme)
	params[FieldTargetAddress] = codec.Address.Encode(accountAddress)
	return e.EntryPointInfo.Message(params)
}

type EPNewL1Deposit struct {
	coreutil.EntryPointInfo[isc.Sandbox]
}

func (e EPNewL1Deposit) Message(depositOriginator isc.AgentID, receiver common.Address, assets *isc.Assets) isc.Message {
	return e.EntryPointInfo.Message(dict.Dict{
		FieldAddress:                  receiver.Bytes(),
		FieldAssets:                   assets.Bytes(),
		FieldAgentIDDepositOriginator: depositOriginator.Bytes(),
	})
}
