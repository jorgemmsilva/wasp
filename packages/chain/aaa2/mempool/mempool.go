package mempool

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/logger"
	iotago "github.com/iotaledger/iota.go/v3"
	consGR "github.com/iotaledger/wasp/packages/chain/aaa2/cons/gr"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/optimism"
	"github.com/iotaledger/wasp/packages/metrics"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
)

type mempool struct {
	chainAddress           iotago.Address
	stateReader            state.OptimisticStateReader
	poolMutex              sync.RWMutex
	timelockedRequestsChan chan isc.OnLedgerRequest
	inPoolCounter          int
	outPoolCounter         int
	pool                   map[isc.RequestID]isc.Request
	ctx                    context.Context
	log                    *logger.Logger
	mempoolMetrics         metrics.MempoolMetrics
}

var _ consGR.Mempool = &mempool{}

func New(
	ctx context.Context,
	chainAddress iotago.Address,
	stateReader state.OptimisticStateReader,
	log *logger.Logger,
	mempoolMetrics metrics.MempoolMetrics,
) Mempool {
	ret := &mempool{
		chainAddress:           chainAddress,
		pool:                   make(map[isc.RequestID]isc.Request),
		ctx:                    ctx,
		log:                    log.Named("mempool"),
		mempoolMetrics:         mempoolMetrics,
		stateReader:            stateReader,
		timelockedRequestsChan: make(chan isc.OnLedgerRequest),
	}
	go ret.addTimelockedRequestsToMempool()
	return ret
}

func (m *mempool) hasBeenProcessed(reqID *isc.RequestID) (hasBeenProcessed bool, err error) {
	err = optimism.RetryOnStateInvalidated(
		func() error {
			hasBeenProcessed, err = blocklog.IsRequestProcessed(m.stateReader.KVStoreReader(), reqID)
			return err
		},
	)
	return hasBeenProcessed, err
}

func shouldBeRemoved(req isc.Request, currentTime time.Time) bool {
	onLedgerReq, ok := req.(isc.OnLedgerRequest)
	if !ok {
		return false
	}

	// Do not process anything with SDRUC for now
	if _, ok := onLedgerReq.Features().ReturnAmount(); ok {
		return true
	}

	return isc.RequestIsExpired(onLedgerReq, currentTime)
}

// isRequestReady return whether a request is unlockable, the result is strictly deterministic
func (m *mempool) isRequestReady(req isc.Request) bool {
	if onLedgerReq, ok := req.(isc.OnLedgerRequest); ok {
		return isc.RequestIsUnlockable(onLedgerReq, m.chainAddress, time.Now())
	}
	return true
}

func (m *mempool) Close() {
	m.ctx.Done()
}

func (m *mempool) GetRequest(id isc.RequestID) isc.Request {
	m.poolMutex.RLock()
	defer m.poolMutex.RUnlock()

	return m.pool[id]
}

func (m *mempool) HasRequest(id isc.RequestID) bool {
	m.poolMutex.RLock()
	defer m.poolMutex.RUnlock()

	_, ok := m.pool[id]
	return ok
}

func (m *mempool) Info(currentTime time.Time) MempoolInfo {
	m.poolMutex.RLock()
	defer m.poolMutex.RUnlock()

	ret := MempoolInfo{
		InPoolCounter:  m.inPoolCounter,
		OutPoolCounter: m.outPoolCounter,
		TotalPool:      len(m.pool),
	}
	return ret
}

func (m *mempool) addTimelockedRequestsToMempool() {
	timelockedRequests := make(map[isc.RequestID]isc.OnLedgerRequest)
	var nextUnlock time.Time
	nextUnlockReqs := make(map[isc.RequestID]isc.Request)
	var timeUntilNextUnlock time.Duration

	for {
		if nextUnlock.IsZero() {
			timeUntilNextUnlock = math.MaxInt64
		} else {
			timeUntilNextUnlock = time.Until(nextUnlock)
		}

		select {
		case req := <-m.timelockedRequestsChan:
			timelockedRequests[req.ID()] = req
			// if this request unlocks before `nextUnlock`, update nextUnlock and nextUnlockRequests
			timelock := req.Output().UnlockConditionSet().Timelock()
			if timelock == nil {
				panic("request without timelock shouldn't have been added here")
			}
			unlockTime := time.Unix(int64(timelock.UnixTime), 0)
			if unlockTime.Before(nextUnlock) {
				nextUnlock = unlockTime
				nextUnlockReqs = make(map[isc.RequestID]isc.Request)
				nextUnlockReqs[req.ID()] = req
			}
			if unlockTime.Equal(nextUnlock) {
				nextUnlockReqs[req.ID()] = req
			}

		case <-time.After(timeUntilNextUnlock):
			// try add To pool
			func() {
				m.poolMutex.Lock()
				defer m.poolMutex.Unlock()
				for id, req := range nextUnlockReqs {
					m.addToPoolNoLock(req)
					delete(timelockedRequests, id)
				}
			}()
			// find the next set of requests to be unlockable
			nextUnlock = time.Time{}
			nextUnlockReqs = make(map[isc.RequestID]isc.Request)
			for id, req := range timelockedRequests {
				timelock := time.Unix(int64(req.Output().UnlockConditionSet().Timelock().UnixTime), 0)
				if nextUnlock.IsZero() || timelock.Before(nextUnlock) {
					nextUnlock = timelock
					nextUnlockReqs = make(map[isc.RequestID]isc.Request)
					nextUnlockReqs[id] = req
					continue
				}
				if timelock.Equal(nextUnlock) {
					nextUnlockReqs[id] = req
				}
			}
		}
	}
}

// adds a request to the pool after doing some basic checks, returns whether it was added successfully
func (m *mempool) addToPoolNoLock(req isc.Request) bool {
	if shouldBeRemoved(req, time.Now()) {
		return false // if expired or shouldn't even be processed, don't add to mempool
	}
	// checking in the state if request is processed.
	reqid := req.ID()
	alreadyProcessed, err := m.hasBeenProcessed(&reqid)
	if err != nil || alreadyProcessed {
		// could not check if it is processed or not, leave it in the in-buffer
		return false
	}
	m.pool[reqid] = req
	m.log.Debugf("IN MEMPOOL %s (+%d / -%d)", req.ID(), m.inPoolCounter, m.outPoolCounter)
	m.mempoolMetrics.CountRequestIn(req)
	return true
}

func (m *mempool) ReceiveRequests(reqs ...isc.Request) []bool {
	if len(reqs) == 0 {
		return nil
	}
	ret := make([]bool, len(reqs))
	m.poolMutex.Lock()
	defer m.poolMutex.Unlock()
	for i, req := range reqs {
		if onledgerReq, ok := req.(isc.OnLedgerRequest); ok {
			// if the request is timelocked, maybe it shouldn't be added to the mempool right away
			timelock := onledgerReq.Output().UnlockConditionSet().Timelock()
			if timelock != nil {
				expiration := onledgerReq.Output().UnlockConditionSet().Expiration()
				if expiration != nil && timelock.UnixTime >= expiration.UnixTime {
					// can never be processed, just reject
					ret[i] = false
					continue
				}
				if timelock.UnixTime > uint32(time.Now().Unix()) {
					// will be unlockable in the future, add to pool later
					m.timelockedRequestsChan <- onledgerReq
					ret[i] = true
					continue
				}
			}
		} else {
			// TODO share with other peers
		}
		ret[i] = m.addToPoolNoLock(req)
	}
	return ret
}

func (m *mempool) removeFromPoolNoLock(reqID isc.RequestID) {
	m.outPoolCounter++
	delete(m.pool, reqID)
	m.log.Debugf("OUT MEMPOOL %s (+%d / -%d)", reqID, m.inPoolCounter, m.outPoolCounter)
	m.mempoolMetrics.CountRequestOut()
	m.mempoolMetrics.CountBlocksPerChain()
}

func (m *mempool) RemoveRequests(reqs ...isc.RequestID) {
	if len(reqs) == 0 {
		return
	}
	m.poolMutex.Lock()
	defer m.poolMutex.Unlock()

	for _, rid := range reqs {
		if _, ok := m.pool[rid]; !ok {
			continue
		}
		m.removeFromPoolNoLock(rid)
	}
}

// ConsensusProposalsAsync returns a list of requests to be sent as a batch proposal
func (m *mempool) ConsensusProposalsAsync(ctx context.Context, aliasOutput *isc.AliasOutputWithID) <-chan []*isc.RequestRef {
	retChan := make(chan []*isc.RequestRef, 1)

	go func() {
		m.poolMutex.RLock()
		defer m.poolMutex.RUnlock()

		// transverse the mempool, detect expired requests, build the batch proposal
		ret := make([]*isc.RequestRef, 0, len(m.pool))
		toRemove := []isc.RequestID{}
		for _, req := range m.pool {
			if shouldBeRemoved(req, time.Now()) {
				toRemove = append(toRemove, req.ID())
				continue
			}
			if m.isRequestReady(req) {
				ret = append(ret, &isc.RequestRef{
					ID:   req.ID(),
					Hash: isc.RequestHash(req),
				})
			}
		}
		retChan <- ret
		go m.RemoveRequests(toRemove...)
	}()

	return retChan
}

// ConsensusRequestsAsync return a list of requests to be processed
func (m *mempool) ConsensusRequestsAsync(ctx context.Context, requestRefs []*isc.RequestRef) <-chan []isc.Request {
	retChan := make(chan []isc.Request, 1)

	go func() {
		m.poolMutex.RLock()
		defer m.poolMutex.RUnlock()

		ret := make([]isc.Request, len(requestRefs))
		for i, ref := range requestRefs {
			req, ok := m.pool[ref.ID]
			if ok {
				ret[i] = req
			} else {
				panic("TODO fetch requests that are not present in the mempool")
			}
		}
		retChan <- ret
	}()

	return retChan
}

// --------------------------------------------------------------

// implementation of SoloMempool interface
var _ SoloMempool = &mempool{}

const waitMempoolEmptyTimeoutDefault = 5 * time.Second

func (m *mempool) WaitPoolEmpty(timeout ...time.Duration) bool {
	currentTime := time.Now()
	deadline := currentTime.Add(waitMempoolEmptyTimeoutDefault)
	if len(timeout) > 0 {
		deadline = currentTime.Add(timeout[0])
	}
	for {
		if len(m.pool) == 0 {
			return true
		}
		time.Sleep(10 * time.Millisecond)
		if time.Now().After(deadline) {
			return false
		}
	}
}
