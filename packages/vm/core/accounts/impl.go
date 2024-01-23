package accounts

import (
	"math/big"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/core/errors/coreerrors"
	"github.com/iotaledger/wasp/packages/vm/core/evm"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

func CommonAccount() isc.AgentID {
	return isc.NewAgentID(
		&iotago.Ed25519Address{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
	)
}

var Processor = Contract.Processor(nil,
	// funcs
	FuncDeposit.WithHandler(deposit),
	FuncFoundryCreateNew.WithHandler(foundryCreateNew),
	FuncFoundryDestroy.WithHandler(foundryDestroy),
	FuncFoundryModifySupply.WithHandler(foundryModifySupply),
	FuncMintNFT.WithHandler(mintNFT),
	FuncTransferAccountToChain.WithHandler(transferAccountToChain),
	FuncTransferAllowanceTo.WithHandler(transferAllowanceTo),
	FuncWithdraw.WithHandler(withdraw),

	// views
	ViewAccountNFTs.WithHandler(viewAccountNFTs),
	ViewAccountNFTAmount.WithHandler(viewAccountNFTAmount),
	ViewAccountNFTsInCollection.WithHandler(viewAccountNFTsInCollection),
	ViewAccountNFTAmountInCollection.WithHandler(viewAccountNFTAmountInCollection),
	ViewNFTIDbyMintID.WithHandler(viewNFTIDbyMintID),
	ViewAccountFoundries.WithHandler(viewAccountFoundries),
	ViewAccounts.WithHandler(viewAccounts),
	ViewBalance.WithHandler(viewBalance),
	ViewBalanceBaseToken.WithHandler(viewBalanceBaseToken),
	ViewBalanceBaseTokenEVM.WithHandler(viewBalanceBaseTokenEVM),
	ViewBalanceNativeToken.WithHandler(viewBalanceNativeToken),
	ViewFoundryOutput.WithHandler(viewFoundryOutput),
	ViewGetAccountNonce.WithHandler(viewGetAccountNonce),
	ViewGetNativeTokenIDRegistry.WithHandler(viewGetNativeTokenIDRegistry),
	ViewNFTData.WithHandler(viewNFTData),
	ViewTotalAssets.WithHandler(viewTotalAssets),
)

// this expects the origin amount minus SD
func SetInitialState(v isc.SchemaVersion, state kv.KVStore, baseTokensOnAnchor iotago.BaseToken, tokenInfo *api.InfoResBaseToken) {
	// initial load with base tokens from origin anchor output exceeding minimum storage deposit assumption
	CreditToAccount(v, state, CommonAccount(), isc.NewFungibleTokens(baseTokensOnAnchor, nil), isc.ChainID{}, tokenInfo)
}

// deposit is a function to deposit attached assets to the sender's chain account
// It does nothing because assets are already on the sender's account
// Allowance is ignored
func deposit(ctx isc.Sandbox) dict.Dict {
	ctx.Log().LogDebugf("accounts.deposit")
	return nil
}

// transferAllowanceTo moves whole allowance from the caller to the specified account on the chain.
// Can be sent as a request (sender is the caller) or can be called
func transferAllowanceTo(ctx isc.Sandbox, targetAccount isc.AgentID) dict.Dict {
	allowance := ctx.AllowanceAvailable().Clone()
	ctx.TransferAllowedFunds(targetAccount)

	if targetAccount.Kind() != isc.AgentIDKindEthereumAddress {
		return nil // done
	}
	if !ctx.Caller().Equals(ctx.Request().SenderAccount()) {
		return nil // only issue "custom EVM tx" when this function is called directly by the request sender
	}
	// issue a "custom EVM tx" so the funds appear on the explorer
	ctx.Call(
		evm.FuncNewL1Deposit.Message(evm.NewL1DepositRequest{
			DepositOriginator: ctx.Caller(),
			Receiver:          targetAccount.(*isc.EthereumAddressAgentID).EthAddress(),
			Assets:            allowance,
		}),
		nil,
	)
	ctx.Log().LogDebugf("accounts.transferAllowanceTo.success: target: %s\n%s", targetAccount.Bech32(ctx.L1API().ProtocolParameters().Bech32HRP()), ctx.AllowanceAvailable())
	return nil
}

var errCallerMustHaveL1Address = coreerrors.Register("caller must have L1 address").Create()

// withdraw sends the allowed funds to the caller's L1 address,
func withdraw(ctx isc.Sandbox) dict.Dict {
	allowance := ctx.AllowanceAvailable()
	ctx.Log().LogDebugf("accounts.withdraw.begin -- %s", allowance)
	if allowance.IsEmpty() {
		panic(ErrNotEnoughAllowance)
	}
	if len(allowance.NFTs) > 1 {
		panic(ErrTooManyNFTsInAllowance)
	}

	caller := ctx.Caller()
	if _, ok := caller.(*isc.ContractAgentID); ok {
		// cannot withdraw from contract account
		panic(vm.ErrUnauthorized)
	}

	// simple case, caller is not a contract, this is a straightforward withdrawal to L1
	callerAddress, ok := isc.AddressFromAgentID(caller)
	if !ok {
		panic(errCallerMustHaveL1Address)
	}
	remains := ctx.TransferAllowedFunds(ctx.AccountID())
	ctx.Requiref(remains.IsEmpty(), "internal: allowance remains must be empty, but is: %s", remains)
	ctx.Send(isc.RequestParameters{
		TargetAddress: callerAddress,
		Assets:        allowance,
	})
	ctx.Log().LogDebugf("accounts.withdraw.success. Sent to address %s: %s",
		callerAddress.String(),
		allowance.String(),
	)
	return nil
}

// transferAccountToChain transfers the specified allowance from the sender SC's L2
// account on the target chain to the sender SC's L2 account on the origin chain.
//
// Caller must be a contract, and we will transfer the allowance from its L2 account
// on the target chain to its L2 account on the origin chain. This requires that
// this function takes the allowance into custody and in turn sends the assets as
// allowance to the origin chain, where that chain's accounts.TransferAllowanceTo()
// function then transfers it into the caller's L2 account on that chain.
//
// IMPORTANT CONSIDERATIONS:
// 1. The caller contract needs to provide sufficient base tokens in its
// allowance, to cover the gas fee GAS1 for this request.
// Note that this amount depend on the fee structure of the target chain,
// which can be different from the fee structure of the caller's own chain.
//
// 2. The caller contract also needs to provide sufficient base tokens in
// its allowance, to cover the gas fee GAS2 for the resulting request to
// accounts.TransferAllowanceTo() on the origin chain. The caller needs to
// specify this GAS2 amount through the GasReserve parameter.
//
// 3. The caller contract also needs to provide a storage deposit SD with
// this request, holding enough base tokens *independent* of the GAS1 and
// GAS2 amounts.
// Since this storage deposit is dictated by L1 we can use this amount as
// storage deposit for the resulting accounts.TransferAllowanceTo() request,
// where it will be then returned to the caller as part of the transfer.
//
// 4. This means that the caller contract needs to provide at least
// GAS1 + GAS2 + SD base tokens as assets to this request, and provide an
// allowance to the request that is exactly GAS2 + SD + transfer amount.
// Failure to meet these conditions may result in a failed request and
// worst case the assets sent to accounts.TransferAllowanceTo() could be
// irretrievably locked up in an account on the origin chain that belongs
// to the accounts core contract of the target chain.
//
// 5. The caller contract needs to set the gas budget for this request to
// GAS1 to guard against unanticipated changes in the fee structure that
// raise the gas price, otherwise the request could accidentally cannibalize
// GAS2 or even SD, with potential failure and locked up assets as a result.
func transferAccountToChain(ctx isc.Sandbox, gasReserveOpt *uint64) dict.Dict {
	allowance := ctx.AllowanceAvailable()
	ctx.Log().LogDebugf("accounts.transferAccountToChain.begin -- %s", allowance)
	if allowance.IsEmpty() {
		panic(ErrNotEnoughAllowance)
	}
	if len(allowance.NFTs) > 1 {
		panic(ErrTooManyNFTsInAllowance)
	}

	caller := ctx.Caller()
	callerContract, ok := caller.(*isc.ContractAgentID)
	if !ok || callerContract.Hname().IsNil() {
		// caller must be contract
		panic(vm.ErrUnauthorized)
	}

	// if the caller contract is on the same chain the transfer would end up
	// in the same L2 account it is taken from, so we do nothing in that case
	if callerContract.ChainID().Equals(ctx.ChainID()) {
		return nil
	}

	// save the assets to send to the transfer request, as specified by the allowance
	assets := allowance.Clone()

	// deduct the gas reserve GAS2 from the allowance, if possible
	// FIXME: add a solo test for FuncTransferAccountToChain and fix the ParamGasReserve logic
	/*
		if allowance.BaseTokens < gasReserve {
			panic(ErrNotEnoughAllowance)
		}
		allowance.BaseTokens -= gasReserve
	*/

	// Warning: this will transfer all assets into the accounts core contract's L2 account.
	// Be sure everything transfers out again, or assets will be stuck forever.
	ctx.TransferAllowedFunds(ctx.AccountID())

	// Send the specified assets, which should include GAS2 and SD, as part of the
	// accounts.TransferAllowanceTo() request on the origin chain.
	// Note that the assets initially end up in the L2 account of this core accounts
	// contract on the origin chain, from where an allowance of SD plus transfer amount
	// will finally end up in the caller's L2 account on the origin chain.
	ctx.Send(isc.RequestParameters{
		TargetAddress: callerContract.Address(),
		Assets:        assets,
		Metadata: &isc.SendMetadata{
			Message:   FuncTransferAllowanceTo.Message(callerContract),
			Allowance: allowance,
			GasBudget: gas.GasUnits(coreutil.FromOptional(gasReserveOpt, uint64(gas.LimitsDefault.MinGasPerRequest))),
		},
	})
	ctx.Log().LogDebugf("accounts.transferAccountToChain.success. Sent to contract %s: %s",
		callerContract.Bech32(ctx.L1API().ProtocolParameters().Bech32HRP()),
		allowance.String(),
	)
	return nil
}

// - must be enough allowance for the storage deposit
func foundryCreateNew(ctx isc.Sandbox, tokenSchemeOpt *iotago.TokenScheme) dict.Dict {
	ctx.Log().LogDebugf("accounts.foundryCreateNew")

	ts := util.MustTokenScheme(coreutil.FromOptional[iotago.TokenScheme](tokenSchemeOpt, &iotago.SimpleTokenScheme{}))
	ts.MeltedTokens = util.Big0
	ts.MintedTokens = util.Big0

	// create UTXO
	sn, storageDepositConsumed := ctx.Privileged().CreateNewFoundry(ts, nil)
	ctx.Requiref(storageDepositConsumed > 0, "storage deposit Consumed > 0: assert failed")
	// storage deposit for the foundry is taken from the allowance and removed from L2 ledger
	debitBaseTokensFromAllowance(ctx, storageDepositConsumed, ctx.ChainID())

	// add to the ownership list of the account
	addFoundryToAccount(ctx.State(), ctx.Caller(), sn)

	ret := dict.New()
	ret.Set(ParamFoundrySN, codec.Uint32.Encode(sn))
	eventFoundryCreated(ctx, sn)
	return ret
}

var errFoundryWithCirculatingSupply = coreerrors.Register("foundry must have zero circulating supply").Create()

// foundryDestroy destroys foundry if that is possible
func foundryDestroy(ctx isc.Sandbox, sn uint32) dict.Dict {
	ctx.Log().LogDebugf("accounts.foundryDestroy")
	// check if foundry is controlled by the caller
	state := ctx.State()
	caller := ctx.Caller()
	if !hasFoundry(state, caller, sn) {
		panic(vm.ErrUnauthorized)
	}

	accountID, ok := ctx.ChainAccountID()
	ctx.Requiref(ok, "chain AccountID unknown")
	out, _ := GetFoundryOutput(state, sn, accountID)
	simpleTokenScheme := util.MustTokenScheme(out.TokenScheme)
	if !util.IsZeroBigInt(big.NewInt(0).Sub(simpleTokenScheme.MintedTokens, simpleTokenScheme.MeltedTokens)) {
		panic(errFoundryWithCirculatingSupply)
	}

	storageDepositReleased := ctx.Privileged().DestroyFoundry(sn)

	deleteFoundryFromAccount(state, caller, sn)
	DeleteFoundryOutput(state, sn)
	// the storage deposit goes to the caller's account
	CreditToAccount(
		ctx.SchemaVersion(),
		state,
		caller,
		&isc.FungibleTokens{BaseTokens: storageDepositReleased},
		ctx.ChainID(),
		ctx.TokenInfo(),
	)
	eventFoundryDestroyed(ctx, sn)
	return nil
}

// foundryModifySupply inflates (mints) or shrinks supply of token by the foundry, controlled by the caller
func foundryModifySupply(ctx isc.Sandbox, sn uint32, delta *big.Int, destroy bool) dict.Dict {
	if util.IsZeroBigInt(delta) {
		return nil
	}
	state := ctx.State()
	caller := ctx.Caller()
	// check if foundry is controlled by the caller
	if !hasFoundry(state, caller, sn) {
		panic(vm.ErrUnauthorized)
	}

	accountID, ok := ctx.ChainAccountID()
	ctx.Requiref(ok, "chain AccountID unknown")
	out, _ := GetFoundryOutput(state, sn, accountID)
	if out == nil {
		panic(errFoundryNotFound)
	}
	nativeTokenID, err := out.NativeTokenID()
	ctx.RequireNoError(err, "internal")

	// accrue change on the caller's account
	// update native tokens on L2 ledger and transit foundry UTXO
	var storageDepositAdjustment int64
	if deltaAssets := isc.NewEmptyFungibleTokens().AddNativeTokens(nativeTokenID, delta); destroy {
		// take tokens to destroy from allowance
		accountID := ctx.AccountID()
		ctx.TransferAllowedFunds(accountID, isc.NewAssets(0, iotago.NativeTokenSum{
			nativeTokenID: delta,
		}))
		DebitFromAccount(ctx.SchemaVersion(), state, accountID, deltaAssets, ctx.ChainID(), ctx.TokenInfo())
		storageDepositAdjustment = ctx.Privileged().ModifyFoundrySupply(sn, delta.Neg(delta))
	} else {
		CreditToAccount(ctx.SchemaVersion(), state, caller, deltaAssets, ctx.ChainID(), ctx.TokenInfo())
		storageDepositAdjustment = ctx.Privileged().ModifyFoundrySupply(sn, delta)
	}

	// adjust base tokens on L2 due to the possible change in storage deposit
	switch {
	case storageDepositAdjustment < 0:
		// storage deposit is taken from the allowance of the caller
		debitBaseTokensFromAllowance(ctx, iotago.BaseToken(-storageDepositAdjustment), ctx.ChainID())
	case storageDepositAdjustment > 0:
		// storage deposit is returned to the caller account
		CreditToAccount(ctx.SchemaVersion(), state, caller, isc.NewFungibleTokens(iotago.BaseToken(storageDepositAdjustment), nil), ctx.ChainID(), ctx.TokenInfo())
	}
	eventFoundryModified(ctx, sn)
	return nil
}
