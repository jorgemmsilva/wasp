// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

// nodeconn package provides an interface to the L1 node (Hornet).
// This component is responsible for:
//   - Protocol details.
//   - Block reattachments and promotions.
package nodeconn

// drop chaining, use the quick "confimation". Handle rollbacks if they happen
// don't do promotion
// re-attachment (?) - could be needed if tx is pending but block is orphaned - (re-attachment might not work for txs with "context inputs") , let's skip it - might just  be better to let the tx fail and re-build the state-update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/app/shutdown"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/runtime/timeutil"
	"github.com/iotaledger/inx-app/pkg/nodebridge"
	inx "github.com/iotaledger/inx/go"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/iota.go/v4/builder"
	"github.com/iotaledger/iota.go/v4/nodeclient"
	"github.com/iotaledger/wasp/packages/chain"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/util"
)

const (
	indexerPluginAvailableTimeout  = 30 * time.Second
	l1NodeSyncWaitTimeout          = 2 * time.Minute
	blockMetadataCheckTimeout      = 3 * time.Second
	blockMetadataCheckCooldownTime = 100 * time.Millisecond
	inxTimeoutInfo                 = 500 * time.Millisecond
	inxTimeoutBlockMetadata        = 500 * time.Millisecond
	inxTimeoutSubmitBlock          = 60 * time.Second
	inxTimeoutPublishTransaction   = 120 * time.Second
	inxTimeoutIndexerQuery         = 2 * time.Second
	inxTimeoutMilestone            = 2 * time.Second
	inxTimeoutOutput               = 2 * time.Second
	inxTimeoutGetPeers             = 2 * time.Second

	chainsCleanupThresholdRatio              = 50.0
	chainsCleanupThresholdCount              = 10
	pendingTransactionsCleanupThresholdRatio = 50.0
	pendingTransactionsCleanupThresholdCount = 1000
)

var ErrOperationAborted = errors.New("operation was aborted")

type LedgerUpdateHandler func(*nodebridge.LedgerUpdate)

// nodeConnection implements chain.NodeConnection.
// Single Wasp node is expected to connect to a single L1 node, thus
// we expect to have a single instance of this structure.
type nodeConnection struct {
	*logger.WrappedLogger

	ctx             context.Context
	syncedCtx       context.Context
	syncedCtxCancel context.CancelFunc
	chainsLock      sync.RWMutex
	chainsMap       *shrinkingmap.ShrinkingMap[isc.ChainID, *ncChain]
	indexerClient   nodeclient.IndexerClient
	nodeBridge      *nodebridge.NodeBridge
	nodeClient      *nodeclient.Client

	// TODO remove
	// pendingTransactionsMap is a map of sent transactions that are pending.
	// pendingTransactionsMap  *shrinkingmap.ShrinkingMap[iotago.TransactionID, *pendingTransaction]
	// pendingTransactionsLock sync.RWMutex
	// reattachWorkerPool      *workerpool.WorkerPool

	shutdownHandler *shutdown.ShutdownHandler
}

func New(
	ctx context.Context,
	log *logger.Logger,
	nodeBridge *nodebridge.NodeBridge,
	shutdownHandler *shutdown.ShutdownHandler,
) (chain.NodeConnection, error) {
	ctxIndexer, cancelIndexer := context.WithTimeout(ctx, indexerPluginAvailableTimeout)
	defer cancelIndexer()

	indexerClient, err := nodeBridge.Indexer(ctxIndexer)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodeclient indexer: %w", err)
	}

	inxNodeClient, err := nodeBridge.INXNodeClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get inx node client: %w", err)
	}

	syncedCtx, syncedCtxCancel := context.WithCancel(ctx)

	nc := &nodeConnection{
		WrappedLogger:   logger.NewWrappedLogger(log),
		ctx:             nil,
		syncedCtx:       syncedCtx,
		syncedCtxCancel: syncedCtxCancel,
		chainsMap: shrinkingmap.New[isc.ChainID, *ncChain](
			shrinkingmap.WithShrinkingThresholdRatio(chainsCleanupThresholdRatio),
			shrinkingmap.WithShrinkingThresholdCount(chainsCleanupThresholdCount),
		),
		chainsLock:    sync.RWMutex{},
		indexerClient: indexerClient,
		nodeBridge:    nodeBridge,
		nodeClient:    inxNodeClient,
		// TODO remove
		// pendingTransactionsMap: shrinkingmap.New[iotago.TransactionID, *pendingTransaction](
		// 	shrinkingmap.WithShrinkingThresholdRatio(pendingTransactionsCleanupThresholdRatio),
		// 	shrinkingmap.WithShrinkingThresholdCount(pendingTransactionsCleanupThresholdCount),
		// ),
		// pendingTransactionsLock: sync.RWMutex{},
		shutdownHandler: shutdownHandler,
	}

	// TODO remove
	// nc.reattachWorkerPool = workerpool.New("L1 reattachments", workerpool.WithWorkerCount(1))

	return nc, nil
}

func newCtxWithTimeout(ctx context.Context, defaultTimeout time.Duration, timeout ...time.Duration) (context.Context, context.CancelFunc) {
	t := defaultTimeout
	if len(timeout) > 0 {
		t = timeout[0]
	}
	return context.WithTimeout(ctx, t)
}

func waitForL1ToBeConnected(ctx context.Context, log *logger.Logger, nodeClient *nodeclient.Client) error {
	inxGetPeers := func(ctx context.Context) ([]*api.PeerInfo, error) {
		ctx, cancel := context.WithTimeout(ctx, inxTimeoutGetPeers)
		defer cancel()
		mgmClient, err := nodeClient.Management(ctx)
		if err != nil {
			return nil, err
		}
		peersResp, err := mgmClient.Peers(ctx)
		if err != nil {
			return nil, err
		}
		return peersResp.Peers, nil
	}

	getNodeConnected := func() (bool, error) {
		peers, err := inxGetPeers(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to get peers: %w", err)
		}

		// check for at least one connected peer
		for _, peer := range peers {
			if peer.Connected {
				return true, nil
			}
		}

		log.Info("waiting for L1 to be connected to other peers...")
		return false, nil
	}

	ticker := time.NewTicker(1 * time.Second)
	defer timeutil.CleanupTicker(ticker)

	for {
		select {
		case <-ticker.C:
			nodeConnected, err := getNodeConnected()
			if err != nil {
				return err
			}

			if nodeConnected {
				// node is connected to other peers
				return nil
			}

		case <-ctx.Done():
			// context was canceled
			return ErrOperationAborted
		}
	}
}

func waitForL1ToBeSynced(ctx context.Context, log *logger.Logger, nodeClient *nodeclient.Client) error {
	ticker := time.NewTicker(1 * time.Second)
	defer timeutil.CleanupTicker(ticker)

	for {
		select {
		case <-ticker.C:
			healthy, err := nodeClient.Health(ctx)
			if err != nil {
				return err
			}

			if healthy {
				// node is healthy, assumed sync // TODO not clear if ensuring SYNC is possible in 2.0, let's just check for health for now
				return nil
			}

		case <-ctx.Done():
			// context was canceled
			return ErrOperationAborted
		}
	}
}

func (nc *nodeConnection) L1API() iotago.API {
	return nc.nodeClient.LatestAPI()
}

func (nc *nodeConnection) Run(ctx context.Context) error {
	nc.ctx = ctx

	// the node bridge needs to be started before waiting for L1 to become synced,
	// otherwise the NodeStatus would never be updated and "syncAndSetProtocolParameters" would be stuck
	// in an infinite loop
	go func() {
		nc.nodeBridge.Run(ctx)

		// if the Run function returns before the context was actually canceled,
		// it means that the connection to L1 node must have failed.
		if !errors.Is(ctx.Err(), context.Canceled) {
			nc.shutdownHandler.SelfShutdown("INX connection to node dropped", true)
		}
	}()

	syncAndSetProtocolParameters := func() error {
		ctxWaitNodeSynced, cancelWaitNodeSynced := context.WithTimeout(ctx, l1NodeSyncWaitTimeout)
		defer cancelWaitNodeSynced()

		// make sure the node is connected to at least one other peer
		// otherwise the node status may not reflect the network status
		if err := waitForL1ToBeConnected(ctxWaitNodeSynced, nc.WrappedLogger.Logger(), nc.nodeClient); err != nil {
			return err
		}

		if err := waitForL1ToBeSynced(ctxWaitNodeSynced, nc.WrappedLogger.Logger(), nc.nodeClient); err != nil {
			return err
		}
		return nil
	}

	if err := syncAndSetProtocolParameters(); err != nil {
		return fmt.Errorf("Getting latest L1 protocol parameters failed, error: %w", err)
	}

	// TODO remove
	// nc.reattachWorkerPool.Start()
	go nc.subscribeToLedgerUpdates()
	go nc.subscribeToBlocks()

	// mark the node connection as synced
	nc.syncedCtxCancel()

	<-ctx.Done()
	// TODO remove
	// nc.reattachWorkerPool.Shutdown()
	// nc.reattachWorkerPool.ShutdownComplete.Wait()

	return nil
}

// WaitUntilInitiallySynced waits until the layer 1 node was initially synced.
func (nc *nodeConnection) WaitUntilInitiallySynced(ctx context.Context) error {
	select {
	case <-ctx.Done():
		// the given context was canceled
		return ctx.Err()

	case <-nc.syncedCtx.Done():
		// node was initially synced
		return nil
	}
}

func (nc *nodeConnection) Bech32HRP() iotago.NetworkPrefix {
	return nc.nodeClient.LatestAPI().ProtocolParameters().Bech32HRP()
}

func (nc *nodeConnection) GetL1ProtocolParams() iotago.ProtocolParameters {
	return nc.nodeClient.LatestAPI().ProtocolParameters()
}

func (nc *nodeConnection) subscribeToLedgerUpdates() {
	if err := nc.nodeBridge.ListenToLedgerUpdates(nc.ctx, 0, 0, nc.handleLedgerUpdate); err != nil && !errors.Is(err, io.EOF) {
		nc.LogError(err)
		nc.shutdownHandler.SelfShutdown(
			fmt.Sprintf("INX connection unexpected error: %s", err.Error()),
			true)
		return
	}
	if nc.ctx.Err() == nil {
		// shutdown in case there isn't a shutdown already in progress
		nc.shutdownHandler.SelfShutdown("INX connection closed", true)
	}
}

func (nc *nodeConnection) subscribeToBlocks() {
	if err := nc.nodeBridge.ListenToBlocks(nc.ctx, func() {}, nc.handleBlock); err != nil && !errors.Is(err, io.EOF) {
		nc.LogError(err)
		nc.shutdownHandler.SelfShutdown(
			fmt.Sprintf("INX connection unexpected error: %s", err.Error()),
			true)
		return
	}
	if nc.ctx.Err() == nil {
		// shutdown in case there isn't a shutdown already in progress
		nc.shutdownHandler.SelfShutdown("INX connection closed", true)
	}
}

// TODO remove
// func (nc *nodeConnection) getMilestoneTimestamp(ctx context.Context, msIndex iotago.SlotIndex) (time.Time, error) {
// 	ctx, cancel := newCtxWithTimeout(ctx, inxTimeoutMilestone)
// 	defer cancel()

// 	milestone, err := nc.nodeBridge.Milestone(ctx, msIndex)
// 	if err != nil {
// 		return time.Time{}, err
// 	}

// 	return time.Unix(int64(milestone.Milestone.Timestamp), 0), nil
// }

func (nc *nodeConnection) outputForOutputID(ctx context.Context, outputID iotago.OutputID) (iotago.Output, error) {
	ctx, cancel := newCtxWithTimeout(ctx, inxTimeoutOutput)
	defer cancel()

	resp, err := nc.nodeBridge.Client().ReadOutput(ctx, inx.NewOutputId(outputID))
	if err != nil {
		return nil, err
	}

	switch resp.GetPayload().(type) {
	//nolint:nosnakecase // grpc uses underscores
	case *inx.OutputResponse_Output:
		iotaOutput, err := resp.GetOutput().UnwrapOutput(nc.L1API())
		if err != nil {
			return nil, err
		}
		return iotaOutput, nil

	//nolint:nosnakecase // grpc uses underscores
	case *inx.OutputResponse_Spent:
		iotaOutput, err := resp.GetSpent().GetOutput().UnwrapOutput(nc.L1API())
		if err != nil {
			return nil, err
		}
		return iotaOutput, nil

	default:
		return nil, errors.New("invalid inx.OutputResponse payload type")
	}
}

// TODO remove
// func (nc *nodeConnection) checkPendingTransactions(ledgerUpdate *ledgerUpdate) {
// 	nc.pendingTransactionsLock.Lock()
// 	defer nc.pendingTransactionsLock.Unlock()

// 	nc.pendingTransactionsMap.ForEach(func(txID iotago.TransactionID, pendingTx *pendingTransaction) bool {
// 		inputWasConsumed := false
// 		for _, consumedInput := range pendingTx.ConsumedInputs() {
// 			if _, exists := ledgerUpdate.outputsConsumedMap[consumedInput]; exists {
// 				inputWasConsumed = true

// 				break
// 			}
// 		}

// 		if !inputWasConsumed {
// 			// check if the transaction needs to be reattached
// 			// nc.reattachWorkerPool.Submit(func() {
// 			// 	nc.reattachWorkerpoolFunc(pendingTx)
// 			// })
// 			return true
// 		}

// 		// a referenced input of this transaction was consumed, so the pending transaction is affected by this ledger update.
// 		// => we need to check if the outputs were created, otherwise this is a conflicting transaction.

// 		// we can easily check this by searching for output index 0.
// 		// if this was created, the rest was created as well because transactions are atomic.
// 		txOutputIDIndexZero := iotago.OutputIDFromTransactionIDAndIndex(pendingTx.ID(), 0)

// 		// mark waiting for pending transaction as done
// 		nc.clearPendingTransactionWithoutLocking(pendingTx.ID())

// 		if _, created := ledgerUpdate.outputsCreatedMap[txOutputIDIndexZero]; !created {
// 			// transaction was conflicting
// 			pendingTx.SetConflicting(errors.New("input was used in another transaction"))
// 		} else {
// 			// transaction was confirmed
// 			pendingTx.SetConfirmed()
// 		}

// 		return true
// 	})
// }

func (nc *nodeConnection) triggerChainCallbacks(ledgerUpdate *ledgerUpdate) error {
	nc.chainsLock.RLock()
	defer nc.chainsLock.RUnlock()

	trackedAnchorOutputsCreatedSortedMapByChainID, trackedAnchorOutputsCreatedMapByOutputID, err := filterAndSortAnchorOutputs(nc.chainsMap, ledgerUpdate)
	if err != nil {
		return err
	}

	otherOutputsCreatedMapByChainID := filterOtherOutputs(nc.chainsMap, ledgerUpdate.outputsCreatedMap, trackedAnchorOutputsCreatedMapByOutputID)

	// fire milestone events for every chain
	nc.chainsMap.ForEach(func(_ isc.ChainID, chain *ncChain) bool {
		// the callbacks have to be fired synchronously, we can't guarantee the order of execution of go routines
		chain.HandleMilestone(ledgerUpdate.slot, nc.L1API().TimeProvider().SlotEndTime(ledgerUpdate.slot))
		return true
	})

	// fire the anchor output events in order
	for chainID, anchorOutputsSorted := range trackedAnchorOutputsCreatedSortedMapByChainID {
		ncChain, exists := nc.chainsMap.Get(chainID)
		if !exists {
			continue
		}

		for _, anchorOutputInfo := range anchorOutputsSorted {
			// the callbacks have to be fired synchronously, we can't guarantee the order of execution of go routines
			ncChain.HandleAnchorOutput(ledgerUpdate.slot, anchorOutputInfo)
		}
	}

	// fire events for all other outputs that were received by the chains
	for chainID, outputs := range otherOutputsCreatedMapByChainID {
		ncChain, exists := nc.chainsMap.Get(chainID)
		if !exists {
			continue
		}

		for _, outputInfo := range outputs {
			// the callbacks have to be fired synchronously, we can't guarantee the order of execution of go routines
			ncChain.HandleRequestOutput(ledgerUpdate.slot, outputInfo)
		}
	}

	return nil
}

// TODO remove
// func (nc *nodeConnection) checkReceivedTxPendingAndCancelPoW(block *iotago.Block, txPayload *iotago.Transaction) {
// 	txID, err := txPayload.ID()
// 	if err != nil {
// 		return
// 	}

// 	nc.pendingTransactionsLock.RLock()
// 	pendingTx, has := nc.pendingTransactionsMap.Get(txID)
// 	if !has {
// 		nc.pendingTransactionsLock.RUnlock()
// 		return
// 	}
// 	nc.pendingTransactionsLock.RUnlock()

// 	// some chain is waiting for the received tx payload
// 	// => check the quality of the block and cancel ongoing PoW tasks
// 	blockID, err := block.ID()
// 	if err != nil {
// 		return
// 	}

// 	// asynchronously check the quality of the received block and cancel the PoW if possible
// 	go func() {
// 		ctxWithTimeout, ctxCancel := context.WithTimeout(nc.ctx, blockMetadataCheckTimeout)
// 		defer ctxCancel()

// 		for ctxWithTimeout.Err() == nil {
// 			ctxMetaWithTimeout, ctxMetaCancel := context.WithTimeout(nc.ctx, inxTimeoutBlockMetadata)

// 			metadata, err := nc.nodeBridge.BlockMetadata(ctxMetaWithTimeout, blockID)
// 			if err != nil {
// 				// block not found yet => try again
// 				ctxMetaCancel()

// 				// block not found yet
// 				// => try again after some timeout
// 				time.Sleep(blockMetadataCheckCooldownTime)
// 				continue
// 			}
// 			ctxMetaCancel()

// 			// => check if the block is solid
// 			if !metadata.Solid {
// 				// block not solid yet
// 				// => try again after some timeout
// 				time.Sleep(blockMetadataCheckCooldownTime)
// 				continue
// 			}

// 			// check if the block was already referenced
// 			if metadata.ReferencedBySlotIndex != 0 {
// 				// block with the tracked tx already got referenced, we can abort attachment of the tx
// 				pendingTx.SetPublished(blockID)
// 				break
// 			}

// 			// block not referenced yet
// 			// => check if the quality of the tips is good or if the block can never be referenced
// 			if metadata.ShouldReattach {
// 				// we can abort the block metadata check, but we should not abort our own attachment of the tx
// 				break
// 			}

// 			// block is solid and the quality of the tips seem fine
// 			// => abort our own attachment
// 			pendingTx.SetPublished(blockID)
// 			break
// 		}
// 	}()
// }

type ledgerUpdate struct {
	slot               iotago.SlotIndex
	outputsCreatedMap  map[iotago.OutputID]*isc.OutputInfo
	outputsConsumedMap map[iotago.OutputID]*isc.OutputInfo
}

func (nc *nodeConnection) unwrapLedgerUpdate(update *nodebridge.LedgerUpdate) (*ledgerUpdate, error) {
	var err error

	// TODO update
	// // we need to get the timestamp of the milestone from the node
	// milestoneTimestamp, err := nc.getMilestoneTimestamp(nc.ctx, update.SlotIndex)
	// if err != nil {
	// 	return nil, err
	// }

	outputsConsumed, err := unwrapSpents(update.Consumed, nc.L1API())
	if err != nil {
		return nil, err
	}

	outputsCreated, err := unwrapOutputs(update.Created, nc.L1API())
	if err != nil {
		return nil, err
	}

	// create maps for faster lookup
	// outputs that are created and consumed in the same milestone exist in both maps
	outputsConsumedMap := lo.KeyBy(outputsConsumed, func(output *isc.OutputInfo) iotago.OutputID {
		return output.OutputID
	})

	outputsCreatedMap := make(map[iotago.OutputID]*isc.OutputInfo, len(outputsCreated))
	lo.ForEach(outputsCreated, func(outputInfo *isc.OutputInfo) {
		// update info in case created outputs were also consumed
		if outputInfoConsumed, exists := outputsConsumedMap[outputInfo.OutputID]; exists {
			outputInfo.TransactionIDSpent = outputInfoConsumed.TransactionIDSpent
		}

		outputsCreatedMap[outputInfo.OutputID] = outputInfo
	})

	return &ledgerUpdate{
		slot:               update.Slot,
		outputsCreatedMap:  outputsCreatedMap,
		outputsConsumedMap: outputsConsumedMap,
	}, nil
}

func (nc *nodeConnection) handleLedgerUpdate(update *nodebridge.LedgerUpdate) error {
	// unwrap the ledger update into wasp structs
	ledgerUpdate, err := nc.unwrapLedgerUpdate(update)
	if err != nil {
		return err
	}

	// trigger the callbacks of all affected chains
	if err := nc.triggerChainCallbacks(ledgerUpdate); err != nil {
		return err
	}

	// check if pending transactions were affected by the ledger update
	// nc.checkPendingTransactions(ledgerUpdate)

	return nil
}

func (nc *nodeConnection) handleBlock(block *iotago.Block) {
	if block == nil {
		return
	}

	body, ok := block.Body.(*iotago.BasicBlockBody)
	if !ok {
		return
	}

	// check if the block contains a transaction payload
	_, isTransactionPayload := body.Payload.(*iotago.SignedTransaction)
	if !isTransactionPayload {
		// not a transaction payload
		return
	}

	// TODO check differently here
	// // check if the same tx is being tracked in any of the chains,
	// // and cancel the ongoing PoW if the received tx is attached correctly.
	// nc.checkReceivedTxPendingAndCancelPoW(block, txPayload)
}

// GetChain returns the chain if it was registered, otherwise it returns an error.
func (nc *nodeConnection) GetChain(chainID isc.ChainID) (*ncChain, error) {
	nc.chainsLock.RLock()
	defer nc.chainsLock.RUnlock()

	ncc, exists := nc.chainsMap.Get(chainID)
	if !exists {
		return nil, fmt.Errorf("chain %v is not connected", chainID.String())
	}

	return ncc, nil
}

// doPostTx posts the transaction on layer 1 including tipselection and proof of work.
// this function does not wait until the transaction gets confirmed on L1.
// TODO maybe this should return the TXID instead, so we can check for it
func (nc *nodeConnection) doPostTx(ctx context.Context, tx *iotago.SignedTransaction, strongParents iotago.BlockIDs, weakParents iotago.BlockIDs) (iotago.BlockID, error) {
	if ctx.Err() != nil {
		// context may have already been canceled
		return iotago.EmptyBlockID, ctx.Err()
	}

	// Build a Block and post it.
	block, err := builder.NewBasicBlockBuilder(nc.L1API()).
		StrongParents(strongParents).
		WeakParents(weakParents).
		Payload(tx).
		Build()
	if err != nil {
		return iotago.EmptyBlockID, fmt.Errorf("failed to build a tx: %w", err)
	}

	blockID, err := nc.nodeBridge.SubmitBlock(ctx, block)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			// context was canceled
			return iotago.EmptyBlockID, ctx.Err()
		}
		return iotago.EmptyBlockID, fmt.Errorf("failed to submit a tx: %w", err)
	}

	return blockID, nil
}

// TODO remove
// reattachWorkerpoolFunc is triggered by handleLedgerUpdate for every pending transaction,
// if the inputs of the pending transaction were not consumed in the ledger update.
// func (nc *nodeConnection) reattachWorkerpoolFunc(pendingTx *pendingTransaction) {
// 	if pendingTx.Conflicting() || pendingTx.Confirmed() {
// 		// no need to reattach
// 		// we can remove the tracking of the pending transaction
// 		nc.clearPendingTransaction(pendingTx.transactionID)
// 		return
// 	}

// 	blockID := pendingTx.BlockID()
// 	if blockID.Empty() {
// 		// no need to check because no block was posted by this node yet (maybe busy with doPostTx)
// 		return
// 	}

// 	ctxMetadata, cancelCtxMetadata := context.WithTimeout(nc.ctx, inxTimeoutBlockMetadata)
// 	defer cancelCtxMetadata()

// 	blockMetadata, err := nc.nodeBridge.BlockMetadata(ctxMetadata, blockID)
// 	if err != nil {
// 		// block not found
// 		nc.LogDebugf("reattaching transaction %s failed, error: block not found", pendingTx.ID().ToHex(), blockID.ToHex())
// 		return
// 	}

// 	// check confirmation while we are at it anyway
// 	if blockMetadata.ReferencedBySlotIndex != 0 {
// 		// block was referenced

// 		if blockMetadata.LedgerInclusionState == inx.BlockMetadata_LEDGER_INCLUSION_STATE_INCLUDED {
// 			// block was included => confirmed
// 			pendingTx.SetConfirmed()

// 			return
// 		}

// 		// block was referenced, but not included in the ledger
// 		pendingTx.SetConflicting(fmt.Errorf("tx was not included in the ledger. LedgerInclusionState: %s, ConflictReason: %d", blockMetadata.LedgerInclusionState, blockMetadata.ConflictReason))

// 		return
// 	}

// 	if blockMetadata.ShouldReattach {
// 		pendingTx.Reattach()

// 		return
// 	}

// 	// reattach or promote if needed
// 	if blockMetadata.ShouldPromote {
// 		nc.LogDebugf("promoting transaction %s", pendingTx.ID().ToHex())

// 		ctxSubmitBlock, cancelSubmitBlock := context.WithTimeout(nc.ctx, inxTimeoutSubmitBlock)
// 		defer cancelSubmitBlock()

// 		if err := nc.promoteBlock(ctxSubmitBlock, blockID); err != nil {
// 			nc.LogDebugf("promoting transaction %s failed, error: %w", pendingTx.ID().ToHex(), err)
// 			return
// 		}
// 	}
// }

// TODO remove
// func (nc *nodeConnection) promoteBlock(ctx context.Context, blockID iotago.BlockID) error {
// 	tips, err := nc.nodeBridge.RequestTips(ctx, iotago.BlockMaxParents/2, false)
// 	if err != nil {
// 		return fmt.Errorf("failed to fetch tips: %w", err)
// 	}

// 	// add the blockID we want to promote
// 	tips = append(tips, blockID)

// 	block, err := builder.NewBlockBuilder().Parents(tips).Build()
// 	if err != nil {
// 		return fmt.Errorf("failed to build promotion block: %w", err)
// 	}

// 	if _, err = nc.nodeBridge.SubmitBlock(ctx, block); err != nil {
// 		return fmt.Errorf("failed to submit promotion block: %w", err)
// 	}

// 	return nil
// }

// PublishTX handles promoting and reattachments until the tx is confirmed or the context is canceled.
// Publishing can be canceled via the context.
// The result must be returned via the callback, unless ctx is canceled first.
// It is fine to call the callback, even if the ctx is already canceled.
// PublishTX could be called multiple times in parallel, but only once per chain.
func (nc *nodeConnection) PublishTX(
	ctx context.Context,
	chainID isc.ChainID,
	tx *iotago.SignedTransaction,
	callback chain.TxPostHandler,
) error {
	// check if the chain exists
	ncc, err := nc.GetChain(chainID)
	if err != nil {
		return err
	}

	_, err = nc.doPostTx(ctx, tx, strongParents, weakParents)
	return err

	// pendingTx, err := ncc.createPendingTransaction(ctx, tx)
	// if err != nil {
	// 	return err
	// }

	// // transactions are published asynchronously
	// go func() {
	// 	err = ncc.publishTX(pendingTx)
	// 	if err != nil {
	// 		nc.LogDebug(err.Error())
	// 	}

	// 	// transaction was confirmed if err is nil
	// 	callback(tx, err == nil)
	// }()

	return nil
}

// Alias outputs are expected to be returned in order. Considering the Hornet node, the rules are:
//   - Upon Attach -- existing unspent anchor output is returned FIRST.
//   - Upon receiving a spent/unspent AO from L1 they are returned in
//     the same order, as the milestones are issued.
//   - If a single milestone has several anchor outputs, they have to be ordered
//     according to the chain of TXes.
//
// NOTE: Any out-of-order AO will be considered as a rollback or AO by the chain impl.
func (nc *nodeConnection) AttachChain(
	ctx context.Context, // ctx is the context given by a backgroundworker with PriorityChains, it might get canceled by shutdown signal or "Chains.Deactivate"
	chainID isc.ChainID,
	recvRequestCB chain.RequestOutputHandler,
	recvAnchorOutput chain.AnchorOutputHandler,
	recvMilestone chain.MilestoneHandler,
	onChainConnect func(),
	onChainDisconnect func(),
) {
	chain := func() *ncChain {
		// we need to lock until the chain init is done,
		// otherwise there could be race conditions with new ledger updates in parallel
		nc.chainsLock.Lock()
		defer nc.chainsLock.Unlock()

		chain := newNCChain(ctx, nc, chainID, recvRequestCB, recvAnchorOutput, recvMilestone)

		// the chain is added to the map, even if not synchronzied yet,
		// so we can track all pending ledger updates until the chain is synchronized.
		nc.chainsMap.Set(chainID, chain)
		util.ExecuteIfNotNil(onChainConnect)
		nc.LogDebugf("chain registered: %s = %s", chainID.ShortString(), chainID)

		return chain
	}()

	if err := chain.SyncChainStateWithL1(ctx); err != nil {
		nc.LogError(fmt.Sprintf("synchronizing chain state %s failed: %s", chainID, err.Error()))
		nc.shutdownHandler.SelfShutdown(
			fmt.Sprintf("Cannot sync chain %s with L1, %s", chain.chainID, err.Error()),
			true)
	}

	// disconnect the chain after the context is done
	go func() {
		<-ctx.Done()
		chain.WaitUntilStopped()

		nc.chainsLock.Lock()
		defer nc.chainsLock.Unlock()

		nc.chainsMap.Delete(chainID)
		util.ExecuteIfNotNil(onChainDisconnect)
		nc.LogDebugf("chain unregistered: %s = %s, |remaining|=%v", chainID.ShortString(), chainID, nc.chainsMap.Size())
	}()
}
