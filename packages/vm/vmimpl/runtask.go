package vmimpl

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/iotaledger/hive.go/logger"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/rotate"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/util/panicutil"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/migrations"
	"github.com/iotaledger/wasp/packages/vm/core/migrations/allmigrations"
	"github.com/iotaledger/wasp/packages/vm/vmexceptions"
	"github.com/iotaledger/wasp/packages/vm/vmtxbuilder"
)

func Run(task *vm.VMTask) (res *vm.VMTaskResult, err error) {
	// top exception catcher for all panics
	// The VM session will be abandoned peacefully
	err = panicutil.CatchAllButDBError(func() {
		res = runTask(task)
	}, task.Log)
	if err != nil {
		task.Log.Warnf("GENERAL VM EXCEPTION: the task has been abandoned due to: %s", err.Error())
	}
	return res, err
}

// runTask runs batch of requests on VM
func runTask(task *vm.VMTask) *vm.VMTaskResult {
	if len(task.Requests) == 0 {
		panic("invalid params: must be at least 1 request")
	}

	prevL1Commitment, err := transaction.L1CommitmentFromAnchorOutput(task.Inputs.AnchorOutput)
	if err != nil {
		panic(err)
	}

	stateDraft, err := task.Store.NewStateDraft(task.Timestamp, prevL1Commitment)
	if err != nil {
		panic(err)
	}

	vmctx := &vmContext{
		task:       task,
		stateDraft: stateDraft,
	}

	vmctx.init(prevL1Commitment)

	// run the batch of requests
	requestResults, numSuccess, numOffLedger, unprocessable := vmctx.runRequests(
		vmctx.task.Requests,
		governance.NewStateAccess(stateDraft).MaintenanceStatus(),
		vmctx.task.Log,
	)

	vmctx.assertConsistentGasTotals(requestResults)

	taskResult := &vm.VMTaskResult{
		Task:           task,
		StateDraft:     stateDraft,
		RequestResults: requestResults,
	}

	if !vmctx.task.WillProduceBlock() {
		return taskResult
	}

	numProcessed := uint16(len(requestResults))

	vmctx.task.Log.Debugf("runTask, ran %d requests. success: %d, offledger: %d",
		numProcessed, numSuccess, numOffLedger)

	blockIndex, l1Commitment, timestamp, rotationAddr := vmctx.extractBlock(
		numProcessed, numSuccess, numOffLedger,
		unprocessable,
	)

	vmctx.task.Log.Debugf("closed vmContext: block index: %d, state hash: %s timestamp: %v, rotationAddr: %v",
		blockIndex, l1Commitment, timestamp, rotationAddr)

	taskResult.RotationAddress = rotationAddr
	if taskResult.RotationAddress == nil {
		// rotation does not happen
		taskResult.Transaction, taskResult.Unlocks = vmctx.BuildTransactionEssence(l1Commitment, true)
		vmctx.task.Log.Debugf("runTask OUT. block index: %d", blockIndex)
	} else {
		// rotation happens
		taskResult.Transaction, taskResult.Unlocks, err = rotate.MakeRotationTransactionForSelfManagedChain(
			taskResult.RotationAddress,
			vmctx.task.Inputs,
			vmctx.CreationSlot(),
			task.L1API,
		)
		if err != nil {
			panic(fmt.Sprintf("MakeRotateStateControllerTransaction: %s", err.Error()))
		}
		vmctx.task.Log.Debugf("runTask OUT: rotate to address %s", rotationAddr.String())
	}
	return taskResult
}

func (vmctx *vmContext) init(prevL1Commitment *state.L1Commitment) {
	vmctx.loadChainConfig()

	vmctx.withStateUpdate(func(chainState kv.KVStore) {
		migrationScheme := vmctx.getMigrations()
		vmctx.runMigrations(chainState, migrationScheme)
	})

	// save the AccountID of the AccountOutput
	if id, out, ok := vmctx.task.Inputs.AccountOutput(); ok {
		vmctx.withStateUpdate(func(chainState kv.KVStore) {
			withContractState(chainState, governance.Contract, func(s kv.KVStore) {
				governance.SetChainAccountID(
					s,
					util.AccountIDFromAccountOutput(out, id),
				)
			})
		})
	}

	// save the anchor tx ID of the current state
	vmctx.withStateUpdate(func(chainState kv.KVStore) {
		withContractState(chainState, blocklog.Contract, func(s kv.KVStore) {
			blocklog.UpdateLatestBlockInfo(
				s,
				vmctx.task.Inputs.AnchorOutputID.TransactionID(),
			)
		})
	})

	// save the OutputID of the newly created tokens, foundries and NFTs in the previous block
	vmctx.withStateUpdate(func(chainState kv.KVStore) {
		withContractState(chainState, accounts.Contract, func(s kv.KVStore) {
			accounts.UpdateLatestOutputID(s, vmctx.task.Inputs.AnchorOutputID.TransactionID(), vmctx.task.Inputs.AnchorOutput.StateIndex)
		})
	})

	vmctx.txbuilder = vmtxbuilder.NewAnchorTransactionBuilder(
		vmctx.task.Inputs,
		vmtxbuilder.AccountsContractRead{
			NativeTokenOutput:   vmctx.loadNativeTokenOutput,
			FoundryOutput:       vmctx.loadFoundry,
			NFTOutput:           vmctx.loadNFT,
			TotalFungibleTokens: vmctx.loadTotalFungibleTokens,
		},
	)
}

func (vmctx *vmContext) getMigrations() *migrations.MigrationScheme {
	if vmctx.task.MigrationsOverride != nil {
		return vmctx.task.MigrationsOverride
	}
	return allmigrations.DefaultScheme
}

func (vmctx *vmContext) runRequests(
	reqs []isc.Request,
	maintenanceMode bool,
	log *logger.Logger,
) (
	results []*vm.RequestResult,
	numSuccess uint16,
	numOffLedger uint16,
	unprocessable []isc.OnLedgerRequest,
) {
	results = []*vm.RequestResult{}
	allReqs := slices.Clone(reqs)

	// main loop over the batch of requests
	requestIndexCounter := uint16(0)
	for reqIndex := 0; reqIndex < len(allReqs); reqIndex++ {
		req := allReqs[reqIndex]
		result, unprocessableToRetry, skipReason := vmctx.runRequest(req, requestIndexCounter, maintenanceMode)
		if skipReason != nil {
			if errors.Is(vmexceptions.ErrNotEnoughFundsForSD, skipReason) {
				unprocessable = append(unprocessable, req.(isc.OnLedgerRequest))
			}

			// some requests are just ignored (deterministically)
			log.Infof("request skipped (ignored) by the VM: %s, reason: %v",
				req.ID().String(), skipReason)
			continue
		}

		requestIndexCounter++
		results = append(results, result)
		if req.IsOffLedger() {
			numOffLedger++
		}

		if result.Receipt.Error != nil {
			log.Debugf("runTask, ERROR running request: %s, error: %v", req.ID().String(), result.Receipt.Error)
			continue
		}
		numSuccess++

		isRetry := reqIndex >= len(reqs)
		if isRetry {
			vmctx.removeUnprocessable(req.ID())
		}
		for _, retry := range unprocessableToRetry {
			if len(allReqs) >= math.MaxUint16 {
				log.Warnf("cannot process request to be retried %s (retry requested in %s): too many requests in block",
					retry.ID(), req.ID())
			} else {
				allReqs = append(allReqs, retry)
			}
		}
	}
	return results, numSuccess, numOffLedger, unprocessable
}
