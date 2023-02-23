package accounts

import (
	"math"
	"math/big"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/util"
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
	FuncHarvest.WithHandler(harvest),
	FuncTransferAllowanceTo.WithHandler(transferAllowanceTo),
	FuncWithdraw.WithHandler(withdraw),

	// views
	ViewAccountNFTs.WithHandler(viewAccountNFTs),
	ViewAccountNFTAmount.WithHandler(viewAccountNFTAmount),
	ViewAccountNFTsInCollection.WithHandler(viewAccountNFTsInCollection),
	ViewAccountNFTAmountInCollection.WithHandler(viewAccountNFTAmountInCollection),
	ViewAccountFoundries.WithHandler(viewAccountFoundries),
	ViewAccounts.WithHandler(viewAccounts),
	ViewBalance.WithHandler(viewBalance),
	ViewBalanceBaseToken.WithHandler(viewBalanceBaseToken),
	ViewBalanceNativeToken.WithHandler(viewBalanceNativeToken),
	ViewFoundryOutput.WithHandler(viewFoundryOutput),
	ViewGetAccountNonce.WithHandler(viewGetAccountNonce),
	ViewGetNativeTokenIDRegistry.WithHandler(viewGetNativeTokenIDRegistry),
	ViewNFTData.WithHandler(viewNFTData),
	ViewTotalAssets.WithHandler(viewTotalAssets),
)

// TODO this expects the origin amount minus SD
func SetInitialState(state kv.KVStore, baseTokensOnAnchor uint64) {
	// initial load with base tokens from origin anchor output exceeding minimum storage deposit assumption
	CreditToAccount(state, CommonAccount(), isc.NewAssetsBaseTokens(baseTokensOnAnchor))
}

// deposit is a function to deposit attached assets to the sender's chain account
// It does nothing because assets are already on the sender's account
// Allowance is ignored
func deposit(ctx isc.Sandbox) dict.Dict {
	ctx.Log().Debugf("accounts.deposit")
	return nil
}

// transferAllowanceTo moves whole allowance from the caller to the specified account on the chain.
// Can be sent as a request (sender is the caller) or can be called
// Params:
// - ParamAgentID. AgentID. Required
func transferAllowanceTo(ctx isc.Sandbox) dict.Dict {
	ctx.Log().Debugf("accounts.transferAllowanceTo.begin -- %s", ctx.AllowanceAvailable())
	targetAccount := ctx.Params().MustGetAgentID(ParamAgentID)
	ctx.TransferAllowedFunds(targetAccount)
	ctx.Log().Debugf("accounts.transferAllowanceTo.success: target: %s\n%s", targetAccount, ctx.AllowanceAvailable())
	return nil
}

// TODO this is just a temporary value, we need to make deposits fee constant across chains.
const ConstDepositFeeTmp = 1 * isc.Million

// withdraw sends the allowed funds to the caller's L1 address, or if the caller is a
// cross-chain contract, to its account.
func withdraw(ctx isc.Sandbox) dict.Dict {
	ctx.Requiref(!ctx.AllowanceAvailable().IsEmpty(), "Allowance can't be empty in 'accounts.withdraw'")

	callerAddress, ok := isc.AddressFromAgentID(ctx.Caller())
	ctx.Requiref(ok, "caller must have L1 address")

	callerContract, _ := ctx.Caller().(*isc.ContractAgentID)
	if callerContract != nil && callerContract.ChainID().Equals(ctx.ChainID()) {
		// if the caller is on the same chain, do nothing
		return nil
	}

	// move all allowed funds to the account of the current contract context
	// before saving the allowance budget because after the transfer it is mutated
	allowance := ctx.AllowanceAvailable()
	fundsToWithdraw := allowance
	if len(allowance.NFTs) > 0 {
		if len(allowance.NFTs) > 1 {
			panic(ErrTooManyNFTsInAllowance)
		}
	}
	remains := ctx.TransferAllowedFunds(ctx.AccountID())

	// por las dudas
	ctx.Requiref(remains.IsEmpty(), "internal: allowance left after must be empty")

	if callerContract != nil && !callerContract.Hname().IsNil() {
		// deduct the deposit fee from the allowance, so that there are enough tokens to pay for the deposit on the target chain
		allowance := isc.NewAssetsBaseTokens(fundsToWithdraw.BaseTokens - ConstDepositFeeTmp)
		// send funds to a contract on another chain
		ctx.Send(isc.RequestParameters{
			TargetAddress: callerAddress,
			Assets:        fundsToWithdraw,
			Metadata: &isc.SendMetadata{
				TargetContract: Contract.Hname(),
				EntryPoint:     FuncTransferAllowanceTo.Hname(),
				Allowance:      allowance,
				Params:         dict.Dict{ParamAgentID: codec.EncodeAgentID(callerContract)},
				GasBudget:      math.MaxUint64, // TODO This call will fail if not enough gas, and the funds will be lost (credited to this accounts on the target chain)
			},
		})
	} else {
		ctx.Send(isc.RequestParameters{
			TargetAddress: callerAddress,
			Assets:        fundsToWithdraw,
		})
	}
	ctx.Log().Debugf("accounts.withdraw.success. Sent to address %s: %s",
		callerAddress,
		ctx.AllowanceAvailable().String(),
	)
	return nil
}

// harvest moves all the L2 balances of chain commmon account to chain owner's account
// Params:
//
//	ParamForceMinimumBaseTokens: specify the number of BaseTokens left on the common account will be not less than MinimumBaseTokensOnCommonAccount constant
//
// TODO refactor owner of the chain moves all tokens balance the common account to its own account
func harvest(ctx isc.Sandbox) dict.Dict {
	ctx.RequireCallerIsChainOwner()

	state := ctx.State()

	bottomBaseTokens := ctx.Params().MustGetUint64(ParamForceMinimumBaseTokens, MinimumBaseTokensOnCommonAccount)
	if bottomBaseTokens > MinimumBaseTokensOnCommonAccount {
		bottomBaseTokens = MinimumBaseTokensOnCommonAccount
	}
	toWithdraw := GetAccountFungibleTokens(state, CommonAccount())
	if toWithdraw.BaseTokens <= bottomBaseTokens {
		// below minimum, nothing to withdraw
		return nil
	}
	ctx.Requiref(toWithdraw.BaseTokens > bottomBaseTokens, "assertion failed: toWithdraw.BaseTokens > availableBaseTokens")
	toWithdraw.BaseTokens -= bottomBaseTokens
	MustMoveBetweenAccounts(state, CommonAccount(), ctx.Caller(), toWithdraw)
	return nil
}

// Params:
// - token scheme
// - must be enough allowance for the storage deposit
func foundryCreateNew(ctx isc.Sandbox) dict.Dict {
	ctx.Log().Debugf("accounts.foundryCreateNew")

	tokenScheme := ctx.Params().MustGetTokenScheme(ParamTokenScheme, &iotago.SimpleTokenScheme{})
	ts := util.MustTokenScheme(tokenScheme)
	ts.MeltedTokens = util.Big0
	ts.MintedTokens = util.Big0

	// create UTXO
	sn, storageDepositConsumed := ctx.Privileged().CreateNewFoundry(tokenScheme, nil)
	ctx.Requiref(storageDepositConsumed > 0, "storage deposit Consumed > 0: assert failed")
	// storage deposit for the foundry is taken from the allowance and removed from L2 ledger
	debitBaseTokensFromAllowance(ctx, storageDepositConsumed)

	// add to the ownership list of the account
	addFoundryToAccount(ctx.State(), ctx.Caller(), sn)

	ret := dict.New()
	ret.Set(ParamFoundrySN, util.Uint32To4Bytes(sn))
	return ret
}

// foundryDestroy destroys foundry if that is possible
func foundryDestroy(ctx isc.Sandbox) dict.Dict {
	ctx.Log().Debugf("accounts.foundryDestroy")
	sn := ctx.Params().MustGetUint32(ParamFoundrySN)
	// check if foundry is controlled by the caller
	ctx.Requiref(hasFoundry(ctx.State(), ctx.Caller(), sn), "foundry #%d is not controlled by the caller", sn)

	out, _, _ := GetFoundryOutput(ctx.State(), sn, ctx.ChainID())
	simpleTokenScheme := util.MustTokenScheme(out.TokenScheme)
	ctx.Requiref(util.IsZeroBigInt(big.NewInt(0).Sub(simpleTokenScheme.MintedTokens, simpleTokenScheme.MeltedTokens)), "can't destroy foundry with positive circulating supply")

	storageDepositReleased := ctx.Privileged().DestroyFoundry(sn)

	deleteFoundryFromAccount(ctx.State(), ctx.Caller(), sn)
	DeleteFoundryOutput(ctx.State(), sn)
	// the storage deposit goes to the caller's account
	CreditToAccount(ctx.State(), ctx.Caller(), &isc.Assets{
		BaseTokens: storageDepositReleased,
	})
	return nil
}

// foundryModifySupply inflates (mints) or shrinks supply of token by the foundry, controlled by the caller
// Params:
// - ParamFoundrySN serial number of the foundry
// - ParamSupplyDeltaAbs absolute delta of the supply as big.Int
// - ParamDestroyTokens true if destroy supply, false (default) if mint new supply
// NOTE: ParamDestroyTokens is needed since `big.Int` `Bytes()` function does not serialize the sign, only the absolute value
func foundryModifySupply(ctx isc.Sandbox) dict.Dict {
	sn := ctx.Params().MustGetUint32(ParamFoundrySN)
	delta := new(big.Int).Abs(ctx.Params().MustGetBigInt(ParamSupplyDeltaAbs))
	if util.IsZeroBigInt(delta) {
		return nil
	}
	destroy := ctx.Params().MustGetBool(ParamDestroyTokens, false)
	// check if foundry is controlled by the caller
	ctx.Requiref(hasFoundry(ctx.State(), ctx.Caller(), sn), "foundry #%d is not controlled by the caller", sn)

	out, _, _ := GetFoundryOutput(ctx.State(), sn, ctx.ChainID())
	nativeTokenID, err := out.NativeTokenID()
	ctx.RequireNoError(err, "internal")

	// accrue change on the caller's account
	// update native tokens on L2 ledger and transit foundry UTXO
	var storageDepositAdjustment int64
	if deltaAssets := isc.NewEmptyAssets().AddNativeTokens(nativeTokenID, delta); destroy {
		// take tokens to destroy from allowance
		ctx.TransferAllowedFunds(ctx.AccountID(),
			isc.NewAssets(0, iotago.NativeTokens{
				&iotago.NativeToken{
					ID:     nativeTokenID,
					Amount: delta,
				},
			}),
		)
		DebitFromAccount(ctx.State(), ctx.AccountID(), deltaAssets)
		storageDepositAdjustment = ctx.Privileged().ModifyFoundrySupply(sn, delta.Neg(delta))
	} else {
		CreditToAccount(ctx.State(), ctx.Caller(), deltaAssets)
		storageDepositAdjustment = ctx.Privileged().ModifyFoundrySupply(sn, delta)
	}

	// adjust base tokens on L2 due to the possible change in storage deposit
	switch {
	case storageDepositAdjustment < 0:
		// storage deposit is taken from the allowance of the caller
		debitBaseTokensFromAllowance(ctx, uint64(-storageDepositAdjustment))
	case storageDepositAdjustment > 0:
		// storage deposit is returned to the caller account
		CreditToAccount(ctx.State(), ctx.Caller(), isc.NewAssetsBaseTokens(uint64(storageDepositAdjustment)))
	}
	return nil
}
