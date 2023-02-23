//
//
//
//
//
//

package smGPA

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/wasp/packages/chain/statemanager/smGPA/smGPAUtils"
	"github.com/iotaledger/wasp/packages/chain/statemanager/smGPA/smInputs"
	"github.com/iotaledger/wasp/packages/chain/statemanager/smGPA/smMessages"
	"github.com/iotaledger/wasp/packages/chain/statemanager/smUtils"
	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/util"
)

type blockRequestsWithCommitment struct {
	l1Commitment  *state.L1Commitment
	blockRequests []blockRequest
}

type stateManagerGPA struct {
	log                     *logger.Logger
	chainID                 isc.ChainID
	blockCache              smGPAUtils.BlockCache
	blockRequests           map[state.BlockHash]*blockRequestsWithCommitment
	nodeRandomiser          smUtils.NodeRandomiser
	store                   state.Store
	timers                  StateManagerTimers
	lastGetBlocksTime       time.Time
	lastCleanBlockCacheTime time.Time
	lastCleanRequestsTime   time.Time
	lastBlockRequestID      blockRequestID
}

var _ gpa.GPA = &stateManagerGPA{}

const (
	numberOfNodesToRequestBlockFromConst = 5
)

func New(
	chainID isc.ChainID,
	nr smUtils.NodeRandomiser,
	wal smGPAUtils.BlockWAL,
	store state.Store,
	log *logger.Logger,
	timers StateManagerTimers,
) (gpa.GPA, error) {
	var err error
	smLog := log.Named("gpa")
	blockCache, err := smGPAUtils.NewBlockCache(timers.TimeProvider, wal, smLog)
	if err != nil {
		smLog.Errorf("Error creating block cache: %v", err)
		return nil, err
	}
	result := &stateManagerGPA{
		log:                     smLog,
		chainID:                 chainID,
		blockCache:              blockCache,
		blockRequests:           make(map[state.BlockHash]*blockRequestsWithCommitment),
		nodeRandomiser:          nr,
		store:                   store,
		timers:                  timers,
		lastGetBlocksTime:       time.Time{},
		lastCleanBlockCacheTime: time.Time{},
		lastBlockRequestID:      0,
	}

	return result, nil
}

// -------------------------------------
// Implementation for gpa.GPA interface
// -------------------------------------

func (smT *stateManagerGPA) Input(input gpa.Input) gpa.OutMessages {
	switch inputCasted := input.(type) {
	case *smInputs.ConsensusStateProposal: // From consensus
		return smT.handleConsensusStateProposal(inputCasted)
	case *smInputs.ConsensusDecidedState: // From consensus
		return smT.handleConsensusDecidedState(inputCasted)
	case *smInputs.ConsensusBlockProduced: // From consensus
		return smT.handleConsensusBlockProduced(inputCasted)
	case *smInputs.ChainFetchStateDiff: // From mempool
		return smT.handleChainFetchStateDiff(inputCasted)
	case *smInputs.StateManagerTimerTick: // From state manager go routine
		return smT.handleStateManagerTimerTick(inputCasted.GetTime())
	default:
		smT.log.Warnf("Unknown input received, ignoring it: type=%T, message=%v", input, input)
		return nil // No messages to send
	}
}

func (smT *stateManagerGPA) Message(msg gpa.Message) gpa.OutMessages {
	switch msgCasted := msg.(type) {
	case *smMessages.GetBlockMessage:
		return smT.handlePeerGetBlock(msgCasted.Sender(), msgCasted.GetL1Commitment())
	case *smMessages.BlockMessage:
		return smT.handlePeerBlock(msgCasted.Sender(), msgCasted.GetBlock())
	default:
		smT.log.Warnf("Unknown message received, ignoring it: type=%T, message=%v", msg, msg)
		return nil // No messages to send
	}
}

func (smT *stateManagerGPA) Output() gpa.Output {
	return nil
}

func (smT *stateManagerGPA) StatusString() string {
	return fmt.Sprintf(
		"State manager is waiting for %v blocks; "+
			"last time blocks were requested from peer nodes: %v (every %v); "+
			"last time outdated requests were cleared: %v (every %v); "+
			"last time block cache was cleaned: %v (every %v).",
		len(smT.blockRequests),
		util.TimeOrNever(smT.lastGetBlocksTime), smT.timers.StateManagerGetBlockRetry,
		util.TimeOrNever(smT.lastCleanRequestsTime), smT.timers.StateManagerRequestCleaningPeriod,
		util.TimeOrNever(smT.lastCleanBlockCacheTime), smT.timers.BlockCacheBlockCleaningPeriod,
	)
}

func (smT *stateManagerGPA) UnmarshalMessage(data []byte) (gpa.Message, error) {
	if len(data) < 1 {
		return nil, fmt.Errorf("error unmarshalling message: slice of length %d is too short", len(data))
	}
	var message gpa.Message
	switch data[0] {
	case smMessages.MsgTypeBlockMessage:
		message = smMessages.NewEmptyBlockMessage()
	case smMessages.MsgTypeGetBlockMessage:
		message = smMessages.NewEmptyGetBlockMessage()
	default:
		return nil, fmt.Errorf("error unmarshalling message: message type %v unknown", data[0])
	}
	err := message.UnmarshalBinary(data)
	return message, err
}

// -------------------------------------
// Internal functions
// -------------------------------------

func (smT *stateManagerGPA) handlePeerGetBlock(from gpa.NodeID, commitment *state.L1Commitment) gpa.OutMessages {
	// TODO: [KP] Only accept queries from access nodes.
	fromLog := from.ShortString()
	smT.log.Debugf("Message GetBlock %s received from peer %s", commitment, fromLog)
	block := smT.getBlock(commitment)
	if block == nil {
		smT.log.Debugf("Message GetBlock %s: block not found, peer %s request ignored", commitment, fromLog)
		return nil // No messages to send
	}
	smT.log.Debugf("Message GetBlock %s: block found, sending it to peer %s", commitment, fromLog)
	return gpa.NoMessages().Add(smMessages.NewBlockMessage(block, from))
}

func (smT *stateManagerGPA) handlePeerBlock(from gpa.NodeID, block state.Block) gpa.OutMessages {
	blockCommitment := block.L1Commitment()
	blockHash := blockCommitment.BlockHash()
	fromLog := from.ShortString()
	smT.log.Debugf("Message Block %s received from peer %s", blockCommitment, fromLog)
	requestsWC, ok := smT.blockRequests[blockHash]
	if !ok {
		smT.log.Debugf("Message Block %s: block is not needed, ignoring it", blockCommitment)
		return nil // No messages to send
	}
	requests := requestsWC.blockRequests
	smT.log.Debugf("Message Block %s: %v requests are waiting for the block", blockCommitment, len(requests))
	smT.blockCache.AddBlock(block)
	request := smT.createBlockRequestLocal(blockCommitment, func(_ obtainStateFun) {})
	requests = append(requests, request)
	delete(smT.blockRequests, blockHash)
	for _, request := range requests {
		request.blockAvailable(block)
	}
	previousCommitment := block.PreviousL1Commitment()
	smT.log.Debugf("Message Block %s: tracing previous block %s", blockCommitment, previousCommitment)
	messages, err := smT.traceBlockChain(previousCommitment, requests)
	if err != nil {
		return nil // No messages to send
	}
	smT.log.Debugf("Message Block %s from peer %s handled", blockCommitment, fromLog)
	return messages
}

func (smT *stateManagerGPA) handleConsensusStateProposal(csp *smInputs.ConsensusStateProposal) gpa.OutMessages {
	smT.log.Debugf("Input consensus state proposal %s received...", csp.GetL1Commitment())
	smT.lastBlockRequestID++
	request := newBlockRequestFromConsensusStateProposal(csp, smT.log, smT.lastBlockRequestID)
	messages, err := smT.traceBlockChainByRequest(request)
	if err != nil {
		smT.log.Errorf("Input consensus state proposal %s: error tracing block chain: %v", csp.GetL1Commitment(), err)
		return nil // No messages to send
	}
	smT.log.Debugf("Input consensus state proposal %s handled", csp.GetL1Commitment())
	return messages
}

func (smT *stateManagerGPA) handleConsensusDecidedState(cds *smInputs.ConsensusDecidedState) gpa.OutMessages {
	smT.log.Debugf("Input consensus decided state %s received...", cds.GetL1Commitment())
	smT.lastBlockRequestID++
	messages, err := smT.traceBlockChainByRequest(newBlockRequestFromConsensusDecidedState(cds, smT.log, smT.lastBlockRequestID))
	if err != nil {
		smT.log.Errorf("Input consensus decided state %s: error tracing block chain: %v", cds.GetL1Commitment(), err)
		return nil // No messages to send
	}
	smT.log.Debugf("Input consensus decided state %s handled", cds.GetL1Commitment())
	return messages
}

func (smT *stateManagerGPA) handleConsensusBlockProduced(input *smInputs.ConsensusBlockProduced) gpa.OutMessages {
	commitment := input.GetStateDraft().BaseL1Commitment()
	smT.log.Debugf("Input block produced on state %s received...", commitment)
	if !smT.store.HasTrieRoot(commitment.TrieRoot()) {
		smT.log.Panicf("Input block produced on state %s: state, on which this block is produced, is not yet in the store", commitment)
	}
	// NOTE: committing already committed block is allowed (see `TestDoubleCommit` test in `packages/state/state_test.go`)
	block := smT.store.Commit(input.GetStateDraft())
	blockCommitment := block.L1Commitment()
	smT.log.Debugf("Input block produced on state %s: state draft index %v has been committed to the store, resulting block %s",
		commitment, input.GetStateDraft().BlockIndex(), blockCommitment)
	smT.blockCache.AddBlock(block)
	input.Respond(nil)
	requestsWC, ok := smT.blockRequests[blockCommitment.BlockHash()]
	if ok {
		requests := requestsWC.blockRequests
		smT.log.Debugf("Input block produced on state %s: sending block %s to %v requests, waiting for it",
			commitment, len(requestsWC.blockRequests))
		delete(smT.blockRequests, blockCommitment.BlockHash())
		for _, request := range requests {
			request.blockAvailable(block)
		}
		smT.log.Debugf("Input block produced on state %s: completing %v requests",
			commitment, len(requestsWC.blockRequests))
		err := smT.completeRequests(blockCommitment, requests)
		if err != nil {
			smT.log.Errorf("Input block produced on state %s: failed completing %v: %v",
				commitment, len(requestsWC.blockRequests), err)
		}
	}
	smT.log.Debugf("Input block produced on state %s handled", commitment)
	return nil // No messages to send
}

func (smT *stateManagerGPA) handleChainFetchStateDiff(input *smInputs.ChainFetchStateDiff) gpa.OutMessages { //nolint:funlen
	oldCommitment := input.GetOldL1Commitment()
	newCommitment := input.GetNewL1Commitment()
	oldBaseIndex := input.GetOldStateIndex()
	newBaseIndex := input.GetNewStateIndex()
	smT.log.Debugf("Input mempool state request for state (index %v, %s) is received compared to state (index %v, %s)...",
		newBaseIndex, newCommitment, oldBaseIndex, oldCommitment)
	oldNewContainer := &struct {
		oldBlockRequest          *blockRequestImpl
		newBlockRequest          *blockRequestImpl
		oldBlockRequestCompleted bool
		newBlockRequestCompleted bool
		obtainNewStateFun        obtainStateFun
	}{
		oldBlockRequestCompleted: false,
		newBlockRequestCompleted: false,
	}
	isValidFun := func() bool { return input.IsValid() }
	obtainCommittedBlockFun := func(commitment *state.L1Commitment) state.Block {
		result, err := smT.store.BlockByTrieRoot(commitment.TrieRoot())
		if err != nil {
			smT.log.Panicf("Input mempool state request for state (index %v, %s): cannot obtain block %s: %v", newBaseIndex, newCommitment, commitment, err)
		}
		return result
	}
	respondFun := func() {
		var commonIndex uint32
		if newBaseIndex > oldBaseIndex {
			commonIndex = oldBaseIndex
		} else {
			commonIndex = newBaseIndex
		}
		smT.log.Debugf("Input mempool state request for state (index %v, %s): old request base index: %v, new request base index: %v, largest possible common index: %v",
			newBaseIndex, newCommitment, oldBaseIndex, newBaseIndex, commonIndex)

		oldCOB := oldNewContainer.oldBlockRequest.getChainOfBlocks(oldBaseIndex, obtainCommittedBlockFun)
		newCOB := oldNewContainer.newBlockRequest.getChainOfBlocks(newBaseIndex, obtainCommittedBlockFun)

		respondToMempoolFun := func(index uint32) {
			smT.log.Debugf("Input mempool state request for state (index %v, %s): responding by blocks from index %v", newBaseIndex, newCommitment, index)
			newState, err := oldNewContainer.obtainNewStateFun()
			if err != nil {
				smT.log.Errorf("Input mempool state request for state (index %v, %s): unable to obtain new state: %v", newBaseIndex, newCommitment, err)
				return
			}

			input.Respond(smInputs.NewChainFetchStateDiffResults(
				newState,
				newCOB.getBlocksFrom(index),
				oldCOB.getBlocksFrom(index),
			))
		}

		for commonIndex > 0 {
			oldCommonCommitment := oldCOB.getL1Commitment(commonIndex)
			newCommonCommitment := newCOB.getL1Commitment(commonIndex)
			if oldCommonCommitment.Equals(newCommonCommitment) {
				smT.log.Debugf("Input mempool state request for state (index %v, %s): old and new blocks index %v match: %s",
					newBaseIndex, newCommitment, commonIndex, newCommonCommitment)
				respondToMempoolFun(commonIndex)
				return
			}
			smT.log.Debugf("Input mempool state request for state (index %v, %s): old (%s) and new (%s) blocks index %v do not match",
				newBaseIndex, newCommitment, oldCommonCommitment, newCommonCommitment, commonIndex)
			commonIndex--
		}
		respondToMempoolFun(0)
	}
	respondIfNeededFun := func() {
		if oldNewContainer.oldBlockRequestCompleted && oldNewContainer.newBlockRequestCompleted {
			smT.log.Debugf("Input mempool state request for state (index %v, %s): both requests are completed, responding", newBaseIndex, newCommitment)
			respondFun()
		}
	}
	respondFromOldFun := func(_ obtainStateFun) {
		smT.log.Debugf("Input mempool state request for state (index %v, %s): old block request completed", newBaseIndex, newCommitment)
		oldNewContainer.oldBlockRequestCompleted = true
		respondIfNeededFun()
	}
	respondFromNewFun := func(obtainStateFun obtainStateFun) {
		smT.log.Debugf("Input mempool state request for state (index %v, %s): new block request completed", newBaseIndex, newCommitment)
		oldNewContainer.newBlockRequestCompleted = true
		oldNewContainer.obtainNewStateFun = obtainStateFun
		respondIfNeededFun()
	}
	smT.lastBlockRequestID++
	idOld := smT.lastBlockRequestID
	smT.lastBlockRequestID++
	idNew := smT.lastBlockRequestID
	oldNewContainer.oldBlockRequest = newBlockRequestFromMempool("old", input.GetOldL1Commitment(), isValidFun, respondFromOldFun, smT.log, idOld)
	oldNewContainer.newBlockRequest = newBlockRequestFromMempool("new", input.GetNewL1Commitment(), isValidFun, respondFromNewFun, smT.log, idNew)
	smT.log.Debugf("Input mempool state request for state (index %v, %s): requests to fetch new (id %v) and old (id %v) blocks were created",
		newBaseIndex, newCommitment, idNew, idOld)

	result := gpa.NoMessages()
	smT.log.Debugf("Input mempool state request for state (index %v, %s): tracing chain by old block request", newBaseIndex, newCommitment)
	messages, err := smT.traceBlockChainByRequest(oldNewContainer.oldBlockRequest)
	if err != nil {
		smT.log.Errorf("Input mempool state request for state (index %v, %s): error tracing chain by old block request: %v", newBaseIndex, newCommitment, err)
		return nil // No messages to send
	}
	result.AddAll(messages)
	smT.log.Debugf("Input mempool state request for state (index %v, %s): tracing chain by new block request", newBaseIndex, newCommitment)
	messages, err = smT.traceBlockChainByRequest(oldNewContainer.newBlockRequest)
	if err != nil {
		smT.log.Errorf("Input mempool state request for state (index %v, %s): error tracing chain by new block request: %v", newBaseIndex, newCommitment, err)
		return nil // No messages to send
	}
	result.AddAll(messages)
	smT.log.Debugf("Input mempool state request for state (index %v, %s) handled", newBaseIndex, newCommitment)
	return result
}

func (smT *stateManagerGPA) createBlockRequestLocal(commitment *state.L1Commitment, respondFun respondFun) *blockRequestImpl {
	smT.lastBlockRequestID++
	return newBlockRequestLocal(commitment, respondFun, smT.log, smT.lastBlockRequestID)
}

func (smT *stateManagerGPA) getBlock(commitment *state.L1Commitment) state.Block {
	block := smT.blockCache.GetBlock(commitment)
	if block != nil {
		smT.log.Debugf("Block %s retrieved from cache", commitment)
		return block
	}
	smT.log.Debugf("Block %s is not in cache", commitment)

	// Check in store (DB).
	if !smT.store.HasTrieRoot(commitment.TrieRoot()) {
		smT.log.Debugf("Block %s not found in the DB", commitment)
		return nil
	}
	var err error
	block, err = smT.store.BlockByTrieRoot(commitment.TrieRoot())
	if err != nil {
		smT.log.Errorf("Loading block %s from the DB failed: %v", commitment, err)
		return nil
	}
	if !commitment.BlockHash().Equals(block.Hash()) {
		smT.log.Errorf("Block %s loaded from the database has hash %s",
			commitment, block.Hash())
		return nil
	}
	if !commitment.TrieRoot().Equals(block.TrieRoot()) {
		smT.log.Errorf("Block %s loaded from the database has trie root %s",
			commitment, block.TrieRoot())
		return nil
	}
	smT.log.Debugf("Block %s retrieved from the DB", commitment)
	smT.blockCache.AddBlock(block)
	return block
}

func (smT *stateManagerGPA) traceBlockChainByRequest(request blockRequest) (gpa.OutMessages, error) {
	lastCommitment := request.getLastL1Commitment()
	smT.log.Debugf("Request %s id %v tracing block %s chain...", request.getType(), request.getID(), lastCommitment)
	if smT.store.HasTrieRoot(lastCommitment.TrieRoot()) {
		smT.log.Debugf("Request %s id %v tracing block %s chain: the block is already in the store, marking request completed",
			request.getType(), request.getID(), lastCommitment)
		smT.markRequestCompleted(request)
		return nil, nil // No messages to send
	}
	requestsWC, ok := smT.blockRequests[lastCommitment.BlockHash()]
	if ok {
		smT.log.Debugf("Request %s id %v tracing block %s chain: %v request(s) are already waiting for the block; adding this request to the list",
			request.getType(), request.getID(), lastCommitment, len(requestsWC.blockRequests))
		requestsWC.blockRequests = append(requestsWC.blockRequests, request)
		return nil, nil // No messages to send
	}
	return smT.traceBlockChain(lastCommitment, []blockRequest{request})
}

// TODO: state manager may ask for several requests at once: the request can be formulated
//
//	as "give me blocks from some commitment till some index". If the requested
//	node has the required block committed into the store, it certainly has
//	all the blocks before it.
func (smT *stateManagerGPA) traceBlockChain(initCommitment *state.L1Commitment, requests []blockRequest) (gpa.OutMessages, error) {
	smT.log.Debugf("Tracing block %s chain...", initCommitment)
	commitment := initCommitment
	for !smT.store.HasTrieRoot(commitment.TrieRoot()) {
		smT.log.Debugf("Tracing block %s chain: block %s is not in store",
			initCommitment, commitment)
		block := smT.blockCache.GetBlock(commitment)
		if block == nil {
			smT.log.Debugf("Tracing block %s chain: block %s is missing", initCommitment, commitment)
			// Mark that the requests are waiting for `blockHash` block
			blockHash := commitment.BlockHash()
			currentRequestsWC, ok := smT.blockRequests[blockHash]
			if !ok {
				smT.blockRequests[blockHash] = &blockRequestsWithCommitment{
					l1Commitment:  commitment,
					blockRequests: requests,
				}
				smT.log.Debugf("Tracing block %s chain completed: %v requests waiting for block %s, no requests was waiting before",
					initCommitment, len(requests), commitment)
				return smT.makeGetBlockRequestMessages(commitment), nil
			}
			oldLen := len(currentRequestsWC.blockRequests)
			currentRequestsWC.blockRequests = append(currentRequestsWC.blockRequests, requests...)
			smT.log.Debugf("Tracing block %s chain completed: %v requests waiting for block %s is missing, %v requests were waiting for it before",
				initCommitment, len(currentRequestsWC.blockRequests), commitment, oldLen)
			return nil, nil // No messages to send
		}
		for _, request := range requests {
			request.blockAvailable(block)
		}
		commitment = block.PreviousL1Commitment()
	}
	smT.log.Debugf("Tracing block %s chain: tracing completed, completing %v requests", initCommitment, len(requests))
	err := smT.completeRequests(nil, requests)
	smT.log.Debugf("Tracing block %s chain done, err=%v", initCommitment, err)
	return nil, err // No messages to send
}

func (smT *stateManagerGPA) completeRequests(alreadyCommittedL1C *state.L1Commitment, requests []blockRequest) error {
	smT.log.Debugf("Completing %v requests: committing blocks...", len(requests))
	committedBlocks := make(map[state.BlockHash]bool)
	if alreadyCommittedL1C != nil {
		committedBlocks[alreadyCommittedL1C.BlockHash()] = true
	}
	for _, request := range requests {
		smT.log.Debugf("Completing %v requests: committing blocks of %s request %v", len(requests), request.getType(), request.getID())
		blockChain := request.getBlockChain()
		for i := len(blockChain) - 1; i >= 0; i-- {
			block := blockChain[i]
			blockCommitment := block.L1Commitment()
			_, ok := committedBlocks[blockCommitment.BlockHash()]
			if ok {
				smT.log.Debugf("Completing %v requests: block %s is already committed, skipping", len(requests), blockCommitment)
			} else {
				smT.log.Debugf("Completing %v requests: committing block %s...", len(requests), blockCommitment)
				var stateDraft state.StateDraft
				previousCommitment := block.PreviousL1Commitment()
				var err error
				stateDraft, err = smT.store.NewEmptyStateDraft(previousCommitment)
				if err != nil {
					return fmt.Errorf("completing %d requests: error creating empty state draft to store block %s: %w",
						len(requests), blockCommitment, err)
				}
				block.Mutations().ApplyTo(stateDraft)
				committedBlock := smT.store.Commit(stateDraft)
				committedCommitment := committedBlock.L1Commitment()
				if !committedCommitment.Equals(blockCommitment) {
					smT.log.Panicf("Completing %v requests: block, received after committing (%s) differs from the block, which was committed (%s)",
						len(requests), committedCommitment, blockCommitment)
				}
				smT.log.Debugf("Completing %v requests: block index %v %s has been committed to the store on state %s",
					len(requests), stateDraft.BlockIndex(), blockCommitment, previousCommitment)
				committedBlocks[blockCommitment.BlockHash()] = true
			}
		}
	}

	smT.log.Debugf("Completing %v requests: committing blocks completed, marking all the requests as completed", len(requests))
	for _, request := range requests {
		smT.markRequestCompleted(request)
	}
	return nil
}

// Make `numberOfNodesToRequestBlockFromConst` messages to random peers
func (smT *stateManagerGPA) makeGetBlockRequestMessages(commitment *state.L1Commitment) gpa.OutMessages {
	nodeIDs := smT.nodeRandomiser.GetRandomOtherNodeIDs(numberOfNodesToRequestBlockFromConst)
	smT.log.Debugf("Requesting block %s from %v random nodes %s", commitment, numberOfNodesToRequestBlockFromConst, util.SliceShortString(nodeIDs))
	response := gpa.NoMessages()
	for _, nodeID := range nodeIDs {
		response.Add(smMessages.NewGetBlockMessage(commitment, nodeID))
	}
	return response
}

func (smT *stateManagerGPA) markRequestCompleted(request blockRequest) {
	request.markCompleted(func() (state.State, error) {
		return smT.store.StateByTrieRoot(request.getLastL1Commitment().TrieRoot())
	})
}

func (smT *stateManagerGPA) handleStateManagerTimerTick(now time.Time) gpa.OutMessages {
	result := gpa.NoMessages()
	smT.log.Debugf("Input timer tick %v received...", now)
	smT.log.Infof("Status: %s", smT.StatusString())
	nextGetBlocksTime := smT.lastGetBlocksTime.Add(smT.timers.StateManagerGetBlockRetry)
	if now.After(nextGetBlocksTime) {
		smT.log.Debugf("Input timer tick %v: resending get block messages...", now)
		for _, blockRequestsWC := range smT.blockRequests {
			result.AddAll(smT.makeGetBlockRequestMessages(blockRequestsWC.l1Commitment))
		}
		smT.lastGetBlocksTime = now
	} else {
		smT.log.Debugf("Input timer tick %v: no need to resend get block messages; next resend at %v",
			now, nextGetBlocksTime)
	}
	nextCleanBlockCacheTime := smT.lastCleanBlockCacheTime.Add(smT.timers.BlockCacheBlockCleaningPeriod)
	if now.After(nextCleanBlockCacheTime) {
		smT.log.Debugf("Input timer tick %v: cleaning block cache...", now)
		smT.blockCache.CleanOlderThan(now.Add(-smT.timers.BlockCacheBlocksInCacheDuration))
		smT.lastCleanBlockCacheTime = now
	} else {
		smT.log.Debugf("Input timer tick %v: no need to clean block cache; next clean at %v", now, nextCleanBlockCacheTime)
	}
	nextCleanRequestsTime := smT.lastCleanRequestsTime.Add(smT.timers.StateManagerRequestCleaningPeriod)
	if now.After(nextCleanRequestsTime) {
		smT.log.Debugf("Input timer tick %v: cleaning requests...", now)
		newBlockRequestsMap := make(map[state.BlockHash]*blockRequestsWithCommitment)
		for blockHash, blockRequestsWC := range smT.blockRequests {
			commitment := blockRequestsWC.l1Commitment
			blockRequests := blockRequestsWC.blockRequests
			smT.log.Debugf("Input timer tick %v: checking %v requests waiting for block %v",
				now, len(blockRequests), commitment)
			outI := 0
			for _, blockRequest := range blockRequests {
				if blockRequest.isValid() { // Request is valid - keeping it
					blockRequests[outI] = blockRequest
					outI++
				} else {
					smT.log.Debugf("Input timer tick %v: deleting %s request %v as it is no longer valid",
						now, blockRequest.getType(), blockRequest.getID())
				}
			}
			for i := outI; i < len(blockRequests); i++ {
				blockRequests[i] = nil // Not needed requests at the end - freeing memory
			}
			blockRequests = blockRequests[:outI]
			if len(blockRequests) > 0 {
				smT.log.Debugf("Input timer tick %v: %v requests remaining waiting for block %v", now, len(blockRequests), commitment)
				newBlockRequestsMap[blockHash] = &blockRequestsWithCommitment{
					l1Commitment:  commitment,
					blockRequests: blockRequests,
				}
			} else {
				smT.log.Debugf("Input timer tick %v: no more requests waiting for block %v", now, commitment)
			}
		}
		smT.blockRequests = newBlockRequestsMap
		smT.lastCleanRequestsTime = now
	} else {
		smT.log.Debugf("Input timer tick %v: no need to clean requests; next clean at %v", now, nextCleanRequestsTime)
	}
	smT.log.Debugf("Input timer tick %v handled", now)
	return result
}
