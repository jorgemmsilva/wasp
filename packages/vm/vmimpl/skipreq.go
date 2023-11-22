package vmimpl

import (
	"errors"
	"fmt"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/evm/evmutil"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/evm"
	"github.com/iotaledger/wasp/packages/vm/core/evm/evmimpl"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/vmexceptions"
)

// earlyCheckReasonToSkip checks if request must be ignored without even modifying the state
func (reqctx *requestContext) earlyCheckReasonToSkip(maintenanceMode bool) error {
	if reqctx.vm.task.Inputs.AnchorOutput.FeatureSet().NativeToken() != nil {
		if reqctx.vm.task.Inputs.AnchorOutput.StateIndex == 0 {
			return errors.New("can't init chain with native assets on the origin anchor output")
		} else {
			panic("inconsistency: native assets on the anchor output")
		}
	}

	if maintenanceMode &&
		reqctx.req.CallTarget().Contract != governance.Contract.Hname() {
		return errors.New("skipped due to maintenance mode")
	}

	if reqctx.req.IsOffLedger() {
		return reqctx.checkReasonToSkipOffLedger()
	}
	return reqctx.checkReasonToSkipOnLedger()
}

// checkReasonRequestProcessed checks if request ID is already in the blocklog
func (reqctx *requestContext) checkReasonRequestProcessed() error {
	reqid := reqctx.req.ID()
	var isProcessed bool
	withContractState(reqctx.uncommittedState, blocklog.Contract, func(s kv.KVStore) {
		isProcessed = blocklog.MustIsRequestProcessed(s, reqid)
	})
	if isProcessed {
		return errors.New("already processed")
	}
	return nil
}

// checkReasonToSkipOffLedger checks reasons to skip off ledger request
func (reqctx *requestContext) checkReasonToSkipOffLedger() error {
	if reqctx.vm.task.EstimateGasMode {
		return nil
	}
	offledgerReq := reqctx.req.(isc.OffLedgerRequest)
	if err := offledgerReq.VerifySignature(); err != nil {
		return err
	}
	senderAccount := offledgerReq.SenderAccount()

	reqNonce := offledgerReq.Nonce()
	var expectedNonce uint64
	if evmAgentID, ok := senderAccount.(*isc.EthereumAddressAgentID); ok {
		withContractState(reqctx.uncommittedState, evm.Contract, func(s kv.KVStore) {
			expectedNonce = evmimpl.Nonce(s, evmAgentID.EthAddress())
		})
	} else {
		withContractState(reqctx.uncommittedState, accounts.Contract, func(s kv.KVStore) {
			expectedNonce = accounts.AccountNonce(
				s,
				senderAccount,
				reqctx.ChainID(),
			)
		})
	}
	if reqNonce != expectedNonce {
		return fmt.Errorf(
			"invalid nonce (%s): expected %d, got %d",
			offledgerReq.SenderAccount(), expectedNonce, reqNonce,
		)
	}

	if evmTx := offledgerReq.EVMTransaction(); evmTx != nil {
		if err := evmutil.CheckGasPrice(evmTx, reqctx.vm.chainInfo.GasFeePolicy); err != nil {
			return err
		}
	}
	return nil
}

// checkReasonToSkipOnLedger check reasons to skip UTXO request
func (reqctx *requestContext) checkReasonToSkipOnLedger() error {
	if err := reqctx.checkInternalOutput(); err != nil {
		return err
	}
	if err := reqctx.checkReasonReturnAmount(); err != nil {
		return err
	}
	if err := reqctx.checkReasonUnlockable(); err != nil {
		return err
	}
	if reqctx.vm.txbuilder.InputsAreFull() {
		return vmexceptions.ErrInputLimitExceeded
	}
	if err := reqctx.checkReasonRequestProcessed(); err != nil {
		return err
	}
	return nil
}

func (reqctx *requestContext) checkInternalOutput() error {
	// internal outputs are used for internal accounting of assets inside the chain. They are not interpreted as requests
	if reqctx.req.(isc.OnLedgerRequest).IsInternalUTXO(reqctx.ChainID()) {
		return errors.New("it is an internal output")
	}
	return nil
}

// checkReasonUnlockable checks if the request output is unlockable
func (reqctx *requestContext) checkReasonUnlockable() error {
	slot := reqctx.vm.CreationSlot()
	pastBoundedSlotIndex := slot + reqctx.vm.task.L1API.ProtocolParameters().MaxCommittableAge()
	futureBoundedSlotIndex := slot + reqctx.vm.task.L1API.ProtocolParameters().MinCommittableAge()

	req := reqctx.req.(isc.OnLedgerRequest)
	switch out := req.Output().(type) {
	case *iotago.AnchorOutput:
		next := out.Clone().(*iotago.AnchorOutput)
		next.StateIndex += 1
		ok, err := out.UnlockableBy(
			reqctx.vm.ChainID().AsAddress(),
			next,
			pastBoundedSlotIndex,
			futureBoundedSlotIndex,
		)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("output is not unlockable")
		}
	case iotago.TransIndepIdentOutput:
		if !out.UnlockableBy(
			reqctx.vm.ChainID().AsAddress(),
			pastBoundedSlotIndex,
			futureBoundedSlotIndex,
		) {
			return errors.New("output is not unlockable")
		}
	default:
		return fmt.Errorf("no handler for output type %T", out)
	}
	return nil
}

// checkReasonReturnAmount skipping anything with return amounts in this version. There's no risk to lose funds
func (reqctx *requestContext) checkReasonReturnAmount() error {
	if _, ok := reqctx.req.(isc.OnLedgerRequest).Features().ReturnAmount(); ok {
		return errors.New("return amount feature not supported in this version")
	}
	return nil
}
