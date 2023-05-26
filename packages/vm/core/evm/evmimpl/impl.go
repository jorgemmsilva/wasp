// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package evmimpl

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/evm/evmtypes"
	"github.com/iotaledger/wasp/packages/evm/evmutil"
	"github.com/iotaledger/wasp/packages/evm/solidity"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/buffered"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/util/panicutil"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/errors/coreerrors"
	"github.com/iotaledger/wasp/packages/vm/core/evm"
	"github.com/iotaledger/wasp/packages/vm/core/evm/emulator"
	"github.com/iotaledger/wasp/packages/vm/core/evm/iscmagic"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

var Processor = evm.Contract.Processor(nil,
	evm.FuncOpenBlockContext.WithHandler(restricted(openBlockContext)),
	evm.FuncCloseBlockContext.WithHandler(restricted(closeBlockContext)),

	evm.FuncSendTransaction.WithHandler(restricted(applyTransaction)),
	evm.FuncCallContract.WithHandler(restricted(callContract)),

	evm.FuncRegisterERC20NativeToken.WithHandler(registerERC20NativeToken),
	evm.FuncRegisterERC20NativeTokenOnRemoteChain.WithHandler(restricted(registerERC20NativeTokenOnRemoteChain)),
	evm.FuncRegisterERC20ExternalNativeToken.WithHandler(registerERC20ExternalNativeToken),
	evm.FuncRegisterERC721NFTCollection.WithHandler(restricted(registerERC721NFTCollection)),

	// views
	evm.FuncGetERC20ExternalNativeTokenAddress.WithHandler(viewERC20ExternalNativeTokenAddress),
	evm.FuncGetChainID.WithHandler(getChainID),
)

func SetInitialState(state kv.KVStore, evmChainID uint16) {
	// add the standard ISC contract at arbitrary address 0x1074...
	genesisAlloc := core.GenesisAlloc{}
	deployMagicContractOnGenesis(genesisAlloc)

	// add the standard ERC20 contract
	genesisAlloc[iscmagic.ERC20BaseTokensAddress] = core.GenesisAccount{
		Code:    iscmagic.ERC20BaseTokensRuntimeBytecode,
		Storage: map[common.Hash]common.Hash{},
		Balance: nil,
	}
	addToPrivileged(state, iscmagic.ERC20BaseTokensAddress)

	// add the standard ERC721 contract
	genesisAlloc[iscmagic.ERC721NFTsAddress] = core.GenesisAccount{
		Code:    iscmagic.ERC721NFTsRuntimeBytecode,
		Storage: map[common.Hash]common.Hash{},
		Balance: nil,
	}
	addToPrivileged(state, iscmagic.ERC721NFTsAddress)

	// chain always starts with default gas fee & limits configuration
	gasLimits := gas.LimitsDefault
	gasRatio := gas.DefaultFeePolicy().EVMGasRatio
	emulator.Init(
		evmStateSubrealm(state),
		evmChainID,
		emulator.GasLimits{
			Block: gas.EVMBlockGasLimit(gasLimits, &gasRatio),
			Call:  gas.EVMCallGasLimit(gasLimits, &gasRatio),
		},
		0,
		genesisAlloc,
	)

	// subscription to block context is now done in `vmcontext/bootstrapstate.go`
}

var errChainIDMismatch = coreerrors.Register("chainId mismatch").Create()

func applyTransaction(ctx isc.Sandbox) dict.Dict {
	// we only want to charge gas for the actual execution of the ethereum tx
	ctx.Privileged().GasBurnEnable(false)
	defer ctx.Privileged().GasBurnEnable(true)

	tx, err := evmtypes.DecodeTransaction(ctx.Params().Get(evm.FieldTransaction))
	ctx.RequireNoError(err)

	ctx.RequireCaller(isc.NewEthereumAddressAgentID(evmutil.MustGetSender(tx)))

	// next block will be minted when the ISC block is closed
	bctx := getBlockContext(ctx)

	if tx.ChainId().Uint64() != uint64(bctx.emu.BlockchainDB().GetChainID()) {
		panic(errChainIDMismatch)
	}

	tracer := getTracer(ctx, bctx)

	// Send the tx to the emulator.
	// ISC gas burn will be enabled right before executing the tx, and disabled right after,
	// so that ISC magic calls are charged gas.
	receipt, result, iscGasErr, err := bctx.emu.SendTransaction(
		tx,
		ctx,
		tracer,
	)

	if receipt != nil {
		// receipt can be nil when "intrinsic gas too low" or not enough funds
		// If EVM execution was reverted we must revert the ISC request as well.
		// Failed txs will be stored when closing the block context.
		bctx.txs = append(bctx.txs, tx)
		bctx.receipts = append(bctx.receipts, receipt)
	}
	ctx.RequireNoError(err)
	ctx.RequireNoError(iscGasErr)
	ctx.RequireNoError(tryGetRevertError(result))

	return nil
}

var (
	errFoundryNotOwnedByCaller = coreerrors.Register("foundry with serial number %d not owned by caller")
	errEVMAccountAlreadyExists = coreerrors.Register("cannot register ERC20NativeTokens contract: EVM account already exists").Create()
)

func registerERC20NativeToken(ctx isc.Sandbox) dict.Dict {
	foundrySN := codec.MustDecodeUint32(ctx.Params().Get(evm.FieldFoundrySN))
	name := codec.MustDecodeString(ctx.Params().Get(evm.FieldTokenName))
	tickerSymbol := codec.MustDecodeString(ctx.Params().Get(evm.FieldTokenTickerSymbol))
	decimals := codec.MustDecodeUint8(ctx.Params().Get(evm.FieldTokenDecimals))

	{
		res := ctx.CallView(accounts.Contract.Hname(), accounts.ViewAccountFoundries.Hname(), dict.Dict{
			accounts.ParamAgentID: codec.EncodeAgentID(ctx.Caller()),
		})
		if res[kv.Key(codec.EncodeUint32(foundrySN))] == nil {
			panic(errFoundryNotOwnedByCaller.Create(foundrySN))
		}
	}

	// deploy the contract to the EVM state
	addr := iscmagic.ERC20NativeTokensAddress(foundrySN)
	emu := getBlockContext(ctx).emu
	evmState := emu.StateDB()
	if evmState.Exist(addr) {
		panic(errEVMAccountAlreadyExists)
	}
	evmState.CreateAccount(addr)
	evmState.SetCode(addr, iscmagic.ERC20NativeTokensRuntimeBytecode)
	// see ERC20NativeTokens_storage.json
	evmState.SetState(addr, solidity.StorageSlot(0), solidity.StorageEncodeShortString(name))
	evmState.SetState(addr, solidity.StorageSlot(1), solidity.StorageEncodeShortString(tickerSymbol))
	evmState.SetState(addr, solidity.StorageSlot(2), solidity.StorageEncodeUint8(decimals))

	addToPrivileged(ctx.State(), addr)

	return nil
}

var (
	errTargetMustBeAlias   = coreerrors.Register("target must be alias address")
	errOutputMustBeFoundry = coreerrors.Register("expected foundry output")
)

func registerERC20NativeTokenOnRemoteChain(ctx isc.Sandbox) dict.Dict {
	foundrySN := codec.MustDecodeUint32(ctx.Params().Get(evm.FieldFoundrySN))
	name := codec.MustDecodeString(ctx.Params().Get(evm.FieldTokenName))
	tickerSymbol := codec.MustDecodeString(ctx.Params().Get(evm.FieldTokenTickerSymbol))
	decimals := codec.MustDecodeUint8(ctx.Params().Get(evm.FieldTokenDecimals))
	target := codec.MustDecodeAddress(ctx.Params().Get(evm.FieldTargetAddress))
	if target.Type() != iotago.AddressAlias {
		panic(errTargetMustBeAlias)
	}

	{
		res := ctx.CallView(accounts.Contract.Hname(), accounts.ViewAccountFoundries.Hname(), dict.Dict{
			accounts.ParamAgentID: codec.EncodeAgentID(ctx.Caller()),
		})
		if res[kv.Key(codec.EncodeUint32(foundrySN))] == nil {
			panic(errFoundryNotOwnedByCaller.Create(foundrySN))
		}
	}

	tokenScheme := func() iotago.TokenScheme {
		res := ctx.CallView(accounts.Contract.Hname(), accounts.ViewFoundryOutput.Hname(), dict.Dict{
			accounts.ParamFoundrySN: codec.EncodeUint32(foundrySN),
		})
		o := codec.MustDecodeOutput(res[accounts.ParamFoundryOutputBin])
		foundryOutput, ok := o.(*iotago.FoundryOutput)
		if !ok {
			panic(errOutputMustBeFoundry)
		}
		return foundryOutput.TokenScheme
	}()

	req := isc.RequestParameters{
		TargetAddress: target,
		Assets:        isc.NewEmptyAssets(),
		Metadata: &isc.SendMetadata{
			TargetContract: evm.Contract.Hname(),
			EntryPoint:     evm.FuncRegisterERC20ExternalNativeToken.Hname(),
			Params: dict.Dict{
				evm.FieldFoundrySN:          codec.EncodeUint32(foundrySN),
				evm.FieldTokenName:          codec.EncodeString(name),
				evm.FieldTokenTickerSymbol:  codec.EncodeString(tickerSymbol),
				evm.FieldTokenDecimals:      codec.EncodeUint8(decimals),
				evm.FieldFoundryTokenScheme: codec.EncodeTokenScheme(tokenScheme),
			},
		},
	}
	sd := ctx.EstimateRequiredStorageDeposit(req)
	ctx.TransferAllowedFunds(ctx.AccountID(), isc.NewAssetsBaseTokens(sd))
	req.Assets.AddBaseTokens(sd)
	ctx.Send(req)

	return nil
}

var (
	errSenderMustBeAlias            = coreerrors.Register("sender must be alias address").Create()
	errFoundryMustBeOffChain        = coreerrors.Register("foundry must be off-chain").Create()
	errNativeTokenAlreadyRegistered = coreerrors.Register("native token already registered").Create()
)

func registerERC20ExternalNativeToken(ctx isc.Sandbox) dict.Dict {
	caller, ok := ctx.Caller().(*isc.ContractAgentID)
	if !ok {
		panic(errSenderMustBeAlias)
	}
	if ctx.ChainID().Equals(caller.ChainID()) {
		panic(errFoundryMustBeOffChain)
	}
	alias := caller.ChainID().AsAliasAddress()

	name := codec.MustDecodeString(ctx.Params().Get(evm.FieldTokenName))
	tickerSymbol := codec.MustDecodeString(ctx.Params().Get(evm.FieldTokenTickerSymbol))
	decimals := codec.MustDecodeUint8(ctx.Params().Get(evm.FieldTokenDecimals))

	// TODO: We should somehow inspect the real FoundryOutput, but it is on L1.
	// Here we reproduce it from the given params (which we assume to be correct)
	// in order to derive the FoundryID
	foundrySN := codec.MustDecodeUint32(ctx.Params().Get(evm.FieldFoundrySN))
	tokenScheme := codec.MustDecodeTokenScheme(ctx.Params().Get(evm.FieldFoundryTokenScheme))
	simpleTS, ok := tokenScheme.(*iotago.SimpleTokenScheme)
	if !ok {
		panic(errUnsupportedTokenScheme)
	}
	f := &iotago.FoundryOutput{
		SerialNumber: foundrySN,
		TokenScheme:  tokenScheme,
		Conditions: []iotago.UnlockCondition{&iotago.ImmutableAliasUnlockCondition{
			Address: &alias,
		}},
	}
	nativeTokenID, err := f.ID()
	ctx.RequireNoError(err)

	_, ok = getERC20ExternalNativeTokensAddress(ctx, nativeTokenID)
	if ok {
		panic(errNativeTokenAlreadyRegistered)
	}

	emu := getBlockContext(ctx).emu
	evmState := emu.StateDB()

	addr, err := iscmagic.ERC20ExternalNativeTokensAddress(nativeTokenID, evmState.Exist)
	ctx.RequireNoError(err)

	addERC20ExternalNativeTokensAddress(ctx, nativeTokenID, addr)

	// deploy the contract to the EVM state
	evmState.CreateAccount(addr)
	evmState.SetCode(addr, iscmagic.ERC20ExternalNativeTokensRuntimeBytecode)
	// see ERC20ExternalNativeTokens_storage.json
	evmState.SetState(addr, solidity.StorageSlot(0), solidity.StorageEncodeShortString(name))
	evmState.SetState(addr, solidity.StorageSlot(1), solidity.StorageEncodeShortString(tickerSymbol))
	evmState.SetState(addr, solidity.StorageSlot(2), solidity.StorageEncodeUint8(decimals))
	for k, v := range solidity.StorageEncodeBytes(3, nativeTokenID[:]) {
		evmState.SetState(addr, k, v)
	}
	evmState.SetState(addr, solidity.StorageSlot(4), solidity.StorageEncodeUint256(simpleTS.MaximumSupply))

	addToPrivileged(ctx.State(), addr)

	return result(addr[:])
}

func viewERC20ExternalNativeTokenAddress(ctx isc.SandboxView) dict.Dict {
	nativeTokenID := codec.MustDecodeNativeTokenID(ctx.Params().Get(evm.FieldNativeTokenID))
	addr, ok := getERC20ExternalNativeTokensAddress(ctx, nativeTokenID)
	if !ok {
		return nil
	}
	return result(addr[:])
}

func registerERC721NFTCollection(ctx isc.Sandbox) dict.Dict {
	collectionID := codec.MustDecodeNFTID(ctx.Params().Get(evm.FieldNFTCollectionID))

	// The collection NFT must be deposited into the chain before registering. Afterwards it may be
	// withdrawn to L1.
	collection := func() *isc.NFT {
		res := ctx.CallView(accounts.Contract.Hname(), accounts.ViewNFTData.Hname(), dict.Dict{
			accounts.ParamNFTID: codec.EncodeNFTID(collectionID),
		})
		collection, err := isc.NFTFromBytes(res[accounts.ParamNFTData])
		ctx.RequireNoError(err)
		return collection
	}()

	metadata, err := isc.IRC27NFTMetadataFromBytes(collection.Metadata)
	ctx.RequireNoError(err, "cannot decode IRC27 collection NFT metadata")

	// deploy the contract to the EVM state
	addr := iscmagic.ERC721NFTCollectionAddress(collectionID)
	emu := getBlockContext(ctx).emu
	evmState := emu.StateDB()
	if evmState.Exist(addr) {
		panic(errEVMAccountAlreadyExists)
	}
	evmState.CreateAccount(addr)
	evmState.SetCode(addr, iscmagic.ERC721NFTCollectionRuntimeBytecode)
	// see ERC721NFTCollection_storage.json
	evmState.SetState(addr, solidity.StorageSlot(2), solidity.StorageEncodeBytes32(collectionID[:]))
	for k, v := range solidity.StorageEncodeString(3, metadata.Name) {
		evmState.SetState(addr, k, v)
	}

	addToPrivileged(ctx.State(), addr)

	return nil
}

func getChainID(ctx isc.SandboxView) dict.Dict {
	chainID := emulator.GetChainIDFromBlockChainDBState(
		emulator.NewBlockchainDBSubrealm(
			evmStateSubrealm(buffered.NewBufferedKVStore(ctx.StateR())),
		),
	)
	return result(evmtypes.EncodeChainID(chainID))
}

func tryGetRevertError(res *core.ExecutionResult) error {
	// try to include the revert reason in the error
	if res.Err == nil {
		return nil
	}
	if len(res.Revert()) > 0 {
		reason, errUnpack := abi.UnpackRevert(res.Revert())
		if errUnpack == nil {
			return fmt.Errorf("%s: %v", res.Err.Error(), reason)
		}
	}
	return res.Err
}

// callContract is called from the jsonrpc eth_estimateGas and eth_call endpoints.
// The VM is in estimate gas mode, and any state mutations are discarded.
func callContract(ctx isc.Sandbox) dict.Dict {
	// we only want to charge gas for the actual execution of the ethereum tx
	ctx.Privileged().GasBurnEnable(false)
	defer ctx.Privileged().GasBurnEnable(true)

	callMsg, err := evmtypes.DecodeCallMsg(ctx.Params().Get(evm.FieldCallMsg))
	ctx.RequireNoError(err)
	ctx.RequireCaller(isc.NewEthereumAddressAgentID(callMsg.From))

	emu := getBlockContext(ctx).emu

	res, err := emu.CallContract(callMsg, ctx.Privileged().GasBurnEnable)
	ctx.RequireNoError(err)
	ctx.RequireNoError(tryGetRevertError(res))

	gasRatio := getEVMGasRatio(ctx)
	{
		// burn the used EVM gas as it would be done for a normal request call
		ctx.Privileged().GasBurnEnable(true)
		gasErr := panicutil.CatchPanic(
			func() {
				ctx.Gas().Burn(gas.BurnCodeEVM1P, gas.EVMGasToISC(res.UsedGas, &gasRatio))
			},
		)
		ctx.Privileged().GasBurnEnable(false)
		ctx.RequireNoError(gasErr)
	}

	return result(res.ReturnData)
}

func getEVMGasRatio(ctx isc.SandboxBase) util.Ratio32 {
	gasRatioViewRes := ctx.CallView(governance.Contract.Hname(), governance.ViewGetEVMGasRatio.Hname(), nil)
	return codec.MustDecodeRatio32(gasRatioViewRes.Get(governance.ParamEVMGasRatio), gas.DefaultEVMGasRatio)
}
