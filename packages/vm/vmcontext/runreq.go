package vmcontext

import (
	"math"
	"math/big"
	"runtime/debug"
	"time"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/util/panicutil"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/errors/coreerrors"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/packages/vm/gas"
	"github.com/iotaledger/wasp/packages/vm/vmcontext/vmexceptions"
)

// RunTheRequest processes each isc.Request in the batch
func (vmctx *VMContext) RunTheRequest(req isc.Request, requestIndex uint16) (result *vm.RequestResult, err error) {
	// prepare context for the request
	vmctx.req = req
	defer func() { vmctx.req = nil }() // in case `getToBeCaller()` is called afterwards

	vmctx.NumPostedOutputs = 0
	vmctx.requestIndex = requestIndex
	vmctx.requestEventIndex = 0
	vmctx.entropy = hashing.HashData(append(codec.EncodeUint16(requestIndex), vmctx.task.Entropy[:]...))
	vmctx.callStack = vmctx.callStack[:0]
	vmctx.gasBudgetAdjusted = 0
	vmctx.gasBurned = 0
	vmctx.gasFeeCharged = 0
	vmctx.GasBurnEnable(false)

	vmctx.currentStateUpdate = NewStateUpdate()
	vmctx.chainState().Set(kv.Key(coreutil.StatePrefixTimestamp), codec.EncodeTime(vmctx.task.StateDraft.Timestamp().Add(1*time.Nanosecond)))
	defer func() { vmctx.currentStateUpdate = nil }()

	if err2 := vmctx.earlyCheckReasonToSkip(); err2 != nil {
		return nil, err2
	}
	vmctx.loadChainConfig()

	// at this point state update is empty
	// so far there were no panics except optimistic reader
	txsnapshot := vmctx.createTxBuilderSnapshot()

	// catches protocol exception error which is not the request or contract fault
	// If it occurs, the request is just skipped
	err = panicutil.CatchPanicReturnError(
		func() {
			// transfer all attached assets to the sender's account
			vmctx.creditAssetsToChain()
			// load gas and fee policy, calculate and set gas budget
			vmctx.prepareGasBudget()
			// run the contract program
			receipt, callRet := vmctx.callTheContract()
			vmctx.mustCheckTransactionSize()
			result = &vm.RequestResult{
				Request: req,
				Receipt: receipt,
				Return:  callRet,
			}
		}, vmexceptions.AllProtocolLimits...,
	)
	if err != nil {
		// transaction limits exceeded or not enough funds for internal storage deposit. Skipping the request. Rollback
		vmctx.restoreTxBuilderSnapshot(txsnapshot)
		return nil, err
	}
	vmctx.chainState().Apply()
	vmctx.assertConsistentL2WithL1TxBuilder("end RunTheRequest")
	return result, nil
}

// creditAssetsToChain credits L1 accounts with attached assets and accrues all of them to the sender's account on-chain
func (vmctx *VMContext) creditAssetsToChain() {
	vmctx.assertConsistentL2WithL1TxBuilder("begin creditAssetsToChain")

	if vmctx.req.IsOffLedger() {
		// off ledger request does not bring any deposit
		return
	}
	// Consume the output. Adjustment in L2 is needed because of the storage deposit in the internal UTXOs
	storageDepositAdjustment := vmctx.txbuilder.Consume(vmctx.req.(isc.OnLedgerRequest))
	if storageDepositAdjustment > 0 {
		panic("`storageDepositAdjustment > 0`: assertion failed, expected always non-positive storage deposit adjustment")
	}

	// if sender is specified, all assets goes to sender's account
	// Otherwise it all goes to the common account and panics is logged in the SC call
	account := vmctx.req.SenderAccount()
	if account == nil {
		account = accounts.CommonAccount()
	}
	vmctx.creditToAccount(account, vmctx.req.Assets())
	vmctx.creditNFTToAccount(account, vmctx.req.NFT())

	// adjust the sender's account with the storage deposit consumed or returned by internal UTXOs
	// if base tokens in the sender's account is not enough for the storage deposit of newly created TNT outputs
	// it will panic with exceptions.ErrNotEnoughFundsForInternalStorageDeposit
	// TNT outputs will use storage deposit from the caller
	// TODO remove attack vector when base tokens for storage deposit is not enough and the request keeps being skipped
	vmctx.adjustL2BaseTokensIfNeeded(storageDepositAdjustment, account)

	// here transaction builder must be consistent itself and be consistent with the state (the accounts)
	vmctx.assertConsistentL2WithL1TxBuilder("end creditAssetsToChain")
}

// checkAllowance ensure there are enough funds to cover the specified allowance
// panics if not enough funds
func (vmctx *VMContext) checkAllowance() {
	if !vmctx.HasEnoughForAllowance(vmctx.req.SenderAccount(), vmctx.req.Allowance()) {
		panic(vm.ErrNotEnoughFundsForAllowance)
	}
}

func (vmctx *VMContext) prepareGasBudget() {
	if vmctx.req.SenderAccount() == nil {
		return
	}
	vmctx.gasSetBudget(vmctx.calculateAffordableGasBudget())
	vmctx.GasBurnEnable(true)
}

// callTheContract runs the contract. It catches and processes all panics except the one which cancel the whole block
func (vmctx *VMContext) callTheContract() (receipt *blocklog.RequestReceipt, callRet dict.Dict) {
	vmctx.txsnapshot = vmctx.createTxBuilderSnapshot()
	snapMutations := vmctx.currentStateUpdate.Clone()

	if vmctx.req.IsOffLedger() {
		vmctx.updateOffLedgerRequestMaxAssumedNonce()
	}
	var callErr *isc.VMError
	func() {
		defer func() {
			panicErr := vmctx.checkVMPluginPanic(recover())
			if panicErr == nil {
				return
			}
			callErr = panicErr
			vmctx.Debugf("recovered panic from contract call: %v", panicErr)
			if !vmctx.task.EstimateGasMode {
				vmctx.Debugf(string(debug.Stack()))
			}
		}()
		// ensure there are enough funds to cover the specified allowance
		vmctx.checkAllowance()

		callRet = vmctx.callFromRequest()
		// ensure at least the minimum amount of gas is charged
		if vmctx.GasBurned() < gas.BurnCodeMinimumGasPerRequest1P.Cost() {
			vmctx.gasBurnedTotal -= vmctx.gasBurned
			vmctx.gasBurned = 0
			vmctx.GasBurn(gas.BurnCodeMinimumGasPerRequest1P, vmctx.GasBurned())
		}
	}()
	if callErr != nil {
		// panic happened during VM plugin call. Restore the state
		vmctx.restoreTxBuilderSnapshot(vmctx.txsnapshot)
		vmctx.currentStateUpdate = snapMutations
	}
	// charge gas fee no matter what
	vmctx.chargeGasFee()
	// write receipt no matter what
	receipt = vmctx.writeReceiptToBlockLog(callErr)
	return receipt, callRet
}

func (vmctx *VMContext) checkVMPluginPanic(r interface{}) *isc.VMError {
	if r == nil {
		return nil
	}
	// re-panic-ing if error it not user nor VM plugin fault.
	if vmexceptions.IsSkipRequestException(r) {
		panic(r)
	}
	// Otherwise, the panic is wrapped into the returned error, including gas-related panic
	switch err := r.(type) {
	case *isc.VMError:
		return r.(*isc.VMError)
	case isc.VMError:
		e := r.(isc.VMError)
		return &e
	case *kv.DBError:
		panic(err)
	case string:
		return coreerrors.ErrUntypedError.Create(err)
	case error:
		return coreerrors.ErrUntypedError.Create(err.Error())
	}
	return nil
}

// callFromRequest is the call itself. Assumes sc exists
func (vmctx *VMContext) callFromRequest() dict.Dict {
	vmctx.Debugf("callFromRequest: %s", vmctx.req.ID().String())

	if vmctx.req.SenderAccount() == nil {
		// if sender unknown, follow panic path
		panic(vm.ErrSenderUnknown)
	}

	contract := vmctx.req.CallTarget().Contract
	entryPoint := vmctx.req.CallTarget().EntryPoint

	return vmctx.callProgram(
		contract,
		entryPoint,
		vmctx.req.Params(),
		vmctx.req.Allowance(),
	)
}

func (vmctx *VMContext) getGasBudget() uint64 {
	gasBudget, isEVM := vmctx.req.GasBudget()
	if !isEVM {
		return gasBudget
	}

	var gasRatio util.Ratio32
	vmctx.callCore(governance.Contract, func(s kv.KVStore) {
		gasRatio = governance.MustGetGasFeePolicy(s).EVMGasRatio
	})
	return gas.EVMGasToISC(gasBudget, &gasRatio)
}

// calculateAffordableGasBudget checks the account of the sender and calculates affordable gas budget
// Affordable gas budget is calculated from gas budget provided in the request by the user and taking into account
// how many tokens the sender has in its account and how many are allowed for the target.
// Safe arithmetics is used
func (vmctx *VMContext) calculateAffordableGasBudget() uint64 {
	gasBudget := vmctx.getGasBudget()

	// make sure the gasBuget is at least >= than the allowed minimum
	if gasBudget < gas.MinGasPerRequest {
		gasBudget = gas.MinGasPerRequest
	}

	// when estimating gas, if a value bigger than max is provided, use the maximum gas budget possible
	if vmctx.task.EstimateGasMode && gasBudget > gas.MaxGasPerRequest {
		vmctx.gasMaxTokensToSpendForGasFee = math.MaxUint64
		return gas.MaxGasPerRequest
	}

	// calculate how many tokens for gas fee can be guaranteed after taking into account the allowance
	guaranteedFeeTokens := vmctx.calcGuaranteedFeeTokens()
	// calculate how many tokens maximum will be charged taking into account the budget
	f1, f2 := vmctx.chainInfo.GasFeePolicy.FeeFromGasBurned(gasBudget, guaranteedFeeTokens)
	vmctx.gasMaxTokensToSpendForGasFee = f1 + f2
	// calculate affordable gas budget
	affordable := vmctx.chainInfo.GasFeePolicy.GasBudgetFromTokens(guaranteedFeeTokens)
	// adjust gas budget to what is affordable
	affordable = util.MinUint64(gasBudget, affordable)
	// cap gas to the maximum allowed per tx
	return util.MinUint64(affordable, gas.MaxGasPerRequest)
}

// calcGuaranteedFeeTokens return the maximum tokens (base tokens or native) can be guaranteed for the fee,
// taking into account allowance (which must be 'reserved')
// TODO this could be potentially problematic when using custom tokens that are expressed in "big.int"
func (vmctx *VMContext) calcGuaranteedFeeTokens() uint64 {
	var tokensGuaranteed uint64

	if isc.IsEmptyNativeTokenID(vmctx.chainInfo.GasFeePolicy.GasFeeTokenID) {
		// base tokens are used as gas tokens
		tokensGuaranteed = vmctx.GetBaseTokensBalance(vmctx.req.SenderAccount())
		// safely subtract the allowed from the sender to the target
		if allowed := vmctx.req.Allowance(); allowed != nil {
			if tokensGuaranteed < allowed.BaseTokens {
				tokensGuaranteed = 0
			} else {
				tokensGuaranteed -= allowed.BaseTokens
			}
		}
		return tokensGuaranteed
	}
	// native tokens are used for gas fee
	nativeTokenID := vmctx.chainInfo.GasFeePolicy.GasFeeTokenID
	// to pay for gas chain is configured to use some native token, not base tokens
	tokensAvailableBig := vmctx.GetNativeTokenBalance(vmctx.req.SenderAccount(), nativeTokenID)
	if tokensAvailableBig != nil {
		// safely subtract the transfer from the sender to the target
		if transfer := vmctx.req.Allowance(); transfer != nil {
			if transferTokens := transfer.AmountNativeToken(nativeTokenID); !util.IsZeroBigInt(transferTokens) {
				if tokensAvailableBig.Cmp(transferTokens) < 0 {
					tokensAvailableBig.SetUint64(0)
				} else {
					tokensAvailableBig.Sub(tokensAvailableBig, transferTokens)
				}
			}
		}
		if tokensAvailableBig.IsUint64() {
			tokensGuaranteed = tokensAvailableBig.Uint64()
		} else {
			tokensGuaranteed = math.MaxUint64
		}
	}
	return tokensGuaranteed
}

// chargeGasFee takes burned tokens from the sender's account
// It should always be enough because gas budget is set affordable
func (vmctx *VMContext) chargeGasFee() {
	// ensure at least the minimum amount of gas is charged
	minGas := gas.BurnCodeMinimumGasPerRequest1P.Cost()
	if vmctx.GasBurned() < minGas {
		currentGas := vmctx.gasBurned
		vmctx.gasBurned = minGas
		vmctx.gasBurnedTotal += minGas - currentGas
	}

	// disable gas burn
	vmctx.GasBurnEnable(false)
	if vmctx.req.SenderAccount() == nil {
		// no charging if sender is unknown
		return
	}

	availableToPayFee := vmctx.gasMaxTokensToSpendForGasFee
	if !vmctx.task.EstimateGasMode && !vmctx.chainInfo.GasFeePolicy.IsEnoughForMinimumFee(availableToPayFee) {
		// user didn't specify enough base tokens to cover the minimum request fee, charge whatever is present in the user's account
		availableToPayFee = vmctx.GetSenderTokenBalanceForFees()
	}

	// total fees to charge
	sendToOwner, sendToValidator := vmctx.chainInfo.GasFeePolicy.FeeFromGasBurned(vmctx.GasBurned(), availableToPayFee)
	vmctx.gasFeeCharged = sendToOwner + sendToValidator

	// calc gas totals
	vmctx.gasFeeChargedTotal += vmctx.gasFeeCharged

	if vmctx.task.EstimateGasMode {
		// If estimating gas, compute the gas fee but do not attempt to charge
		return
	}

	transferToValidator := &isc.Assets{}
	transferToOwner := &isc.Assets{}
	if !isc.IsEmptyNativeTokenID(vmctx.chainInfo.GasFeePolicy.GasFeeTokenID) {
		transferToValidator.NativeTokens = iotago.NativeTokens{
			&iotago.NativeToken{ID: vmctx.chainInfo.GasFeePolicy.GasFeeTokenID, Amount: big.NewInt(int64(sendToValidator))},
		}
		transferToOwner.NativeTokens = iotago.NativeTokens{
			&iotago.NativeToken{ID: vmctx.chainInfo.GasFeePolicy.GasFeeTokenID, Amount: big.NewInt(int64(sendToOwner))},
		}
	} else {
		transferToValidator.BaseTokens = sendToValidator
		transferToOwner.BaseTokens = sendToOwner
	}
	sender := vmctx.req.SenderAccount()

	vmctx.mustMoveBetweenAccounts(sender, vmctx.task.ValidatorFeeTarget, transferToValidator)
	vmctx.mustMoveBetweenAccounts(sender, accounts.CommonAccount(), transferToOwner)
}

func (vmctx *VMContext) GetContractRecord(contractHname isc.Hname) (ret *root.ContractRecord) {
	ret = vmctx.findContractByHname(contractHname)
	if ret == nil {
		vmctx.GasBurn(gas.BurnCodeCallTargetNotFound)
		panic(vm.ErrContractNotFound.Create(contractHname))
	}
	return ret
}

func (vmctx *VMContext) getOrCreateContractRecord(contractHname isc.Hname) (ret *root.ContractRecord) {
	return vmctx.GetContractRecord(contractHname)
}

// loadChainConfig only makes sense if chain is already deployed
func (vmctx *VMContext) loadChainConfig() {
	vmctx.chainInfo = vmctx.getChainInfo()
	vmctx.chainOwnerID = vmctx.chainInfo.ChainOwnerID
}

// mustCheckTransactionSize panics with ErrMaxTransactionSizeExceeded if the estimated transaction size exceeds the limit
func (vmctx *VMContext) mustCheckTransactionSize() {
	essence, _ := vmctx.txbuilder.BuildTransactionEssence(state.L1CommitmentNil)
	tx := transaction.MakeAnchorTransaction(essence, &iotago.Ed25519Signature{})
	if tx.Size() > parameters.L1().MaxPayloadSize {
		panic(vmexceptions.ErrMaxTransactionSizeExceeded)
	}
}
