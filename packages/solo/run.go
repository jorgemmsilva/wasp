// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package solo

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/iotaledger/hive.go/core/identity"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/rotate"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm"
)

func (ch *Chain) RunOffLedgerRequest(r isc.Request) (dict.Dict, error) {
	defer ch.logRequestLastBlock()
	results := ch.RunRequestsSync([]isc.Request{r}, "off-ledger")
	if len(results) == 0 {
		return nil, errors.New("request was skipped")
	}
	res := results[0]
	var err *isc.UnresolvedVMError
	if !ch.bypassStardustVM {
		// bypass if VM does not implement receipts
		err = res.Receipt.Error
	}
	return res.Return, ch.ResolveVMError(err).AsGoError()
}

func (ch *Chain) RunOffLedgerRequests(reqs []isc.Request) []*vm.RequestResult {
	defer ch.logRequestLastBlock()
	return ch.RunRequestsSync(reqs, "off-ledger")
}

func (ch *Chain) RunRequestsSync(reqs []isc.Request, trace string) (results []*vm.RequestResult) {
	ch.runVMMutex.Lock()
	defer ch.runVMMutex.Unlock()

	ch.mempool.ReceiveRequests(reqs...)

	return ch.runRequestsNolock(reqs, trace)
}

func (ch *Chain) estimateGas(req isc.Request) (result *vm.RequestResult) {
	ch.runVMMutex.Lock()
	defer ch.runVMMutex.Unlock()

	task := ch.runTaskNoLock([]isc.Request{req}, true)
	require.Len(ch.Env.T, task.Results, 1, "cannot estimate gas: request was skipped")
	return task.Results[0]
}

func (ch *Chain) runTaskNoLock(reqs []isc.Request, estimateGas bool) *vm.VMTask {
	anchorOutput := ch.GetAnchorOutput()
	task := &vm.VMTask{
		Processors:         ch.proc,
		AnchorOutput:       anchorOutput.GetAliasOutput(),
		AnchorOutputID:     anchorOutput.OutputID(),
		Requests:           reqs,
		TimeAssumption:     ch.Env.GlobalTime(),
		Store:              ch.store,
		Entropy:            hashing.PseudoRandomHash(nil),
		ValidatorFeeTarget: ch.ValidatorFeeTarget,
		Log:                ch.Log().Desugar().WithOptions(zap.AddCallerSkip(1)).Sugar(),
		// state baseline is always valid in Solo
		EnableGasBurnLogging: true,
		EstimateGasMode:      estimateGas,
	}

	err := ch.vmRunner.Run(task)
	require.NoError(ch.Env.T, err)
	return task
}

func (ch *Chain) runRequestsNolock(reqs []isc.Request, trace string) (results []*vm.RequestResult) {
	ch.Log().Debugf("runRequestsNolock ('%s')", trace)

	task := ch.runTaskNoLock(reqs, false)
	if len(task.Results) == 0 {
		// don't produce empty blocks
		return task.Results
	}

	var essence *iotago.TransactionEssence
	if task.RotationAddress == nil {
		essence = task.ResultTransactionEssence
		copy(essence.InputsCommitment[:], task.ResultInputsCommitment)
	} else {
		var err error
		essence, err = rotate.MakeRotateStateControllerTransaction(
			task.RotationAddress,
			isc.NewAliasOutputWithID(task.AnchorOutput, task.AnchorOutputID),
			task.TimeAssumption.Add(2*time.Nanosecond),
			identity.ID{},
			identity.ID{},
		)
		require.NoError(ch.Env.T, err)
	}
	sigs, err := essence.Sign(
		essence.InputsCommitment[:],
		ch.StateControllerKeyPair.GetPrivateKey().AddressKeys(ch.StateControllerAddress),
	)
	require.NoError(ch.Env.T, err)

	tx := transaction.MakeAnchorTransaction(essence, sigs[0])

	if task.RotationAddress == nil {
		// normal state transition
		ch.settleStateTransition(tx, task.GetProcessedRequestIDs(), task.StateDraft)
	}

	err = ch.Env.AddToLedger(tx)
	require.NoError(ch.Env.T, err)

	anchor, _, err := transaction.GetAnchorFromTransaction(tx)
	require.NoError(ch.Env.T, err)

	if task.RotationAddress != nil {
		ch.Log().Infof("ROTATED STATE CONTROLLER to %s", anchor.StateController)
	}

	rootC := ch.GetRootCommitment()
	l1C := ch.GetL1Commitment()
	require.Equal(ch.Env.T, rootC, l1C.TrieRoot())
	ch.RequestsDone += len(reqs)

	return task.Results
}

func (ch *Chain) settleStateTransition(stateTx *iotago.Transaction, reqids []isc.RequestID, stateDraft state.StateDraft) {
	block := ch.store.Commit(stateDraft)
	err := ch.store.SetLatest(block.TrieRoot())
	if err != nil {
		panic(err)
	}

	ch.Log().Infof("state transition --> #%d. Requests in the block: %d. Outputs: %d",
		stateDraft.BlockIndex(), len(reqids), len(stateTx.Essence.Outputs))
	ch.Log().Debugf("Batch processed: %s", batchShortStr(reqids))

	ch.mempool.RemoveRequests(reqids...)

	go ch.Env.EnqueueRequests(stateTx)
}

func batchShortStr(reqIds []isc.RequestID) string {
	ret := make([]string, len(reqIds))
	for i, r := range reqIds {
		ret[i] = r.Short()
	}
	return fmt.Sprintf("[%s]", strings.Join(ret, ","))
}

func (ch *Chain) logRequestLastBlock() {
	if ch.bypassStardustVM {
		return
	}
	recs := ch.GetRequestReceiptsForBlock(ch.GetLatestBlockInfo().BlockIndex)
	for _, rec := range recs {
		ch.Log().Infof("REQ: '%s'", rec.Short())
	}
}
