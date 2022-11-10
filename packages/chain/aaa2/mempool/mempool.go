package mempool

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/logger"
	iotago "github.com/iotaledger/iota.go/v3"
	consGR "github.com/iotaledger/wasp/packages/chain/aaa2/cons/gr"
	"github.com/iotaledger/wasp/packages/chain/aaa2/mempool/mempoolgpa"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	metrics_pkg "github.com/iotaledger/wasp/packages/metrics"
	"github.com/iotaledger/wasp/packages/peering"
)

const (
	msgTypeMempool byte = iota
)

type mempool struct {
	chainAddress           iotago.Address
	lastSeenChainOutput    *isc.AliasOutputWithID
	poolMutex              sync.RWMutex
	timelockedRequestsChan chan isc.OnLedgerRequest
	inPoolCounter          int
	outPoolCounter         int
	pool                   map[isc.RequestID]isc.Request
	net                    peering.NetworkProvider
	netPeeringID           peering.PeeringID
	peers                  map[gpa.NodeID]*cryptolib.PublicKey
	incomingRequests       *events.Event
	hasBeenProcessed       HasBeenProcessedFunc
	getProcessedRequests   GetProcessedRequestsFunc
	ctx                    context.Context
	log                    *logger.Logger
	metrics                metrics_pkg.MempoolMetrics
	gpa                    *mempoolgpa.Impl
}

var _ consGR.Mempool = &mempool{}

func New(
	ctx context.Context,
	chainID *isc.ChainID,
	myNodeID gpa.NodeID,
	net peering.NetworkProvider,
	hasBeenProcessed HasBeenProcessedFunc,
	getProcessedRequests GetProcessedRequestsFunc,
	log *logger.Logger,
	mempoolMetrics metrics_pkg.MempoolMetrics,
) Mempool {
	pool := &mempool{
		chainAddress:           chainID.AsAddress(),
		pool:                   make(map[isc.RequestID]isc.Request),
		net:                    net,
		hasBeenProcessed:       hasBeenProcessed,
		getProcessedRequests:   getProcessedRequests,
		ctx:                    ctx,
		log:                    log.Named("mempool"),
		metrics:                mempoolMetrics,
		timelockedRequestsChan: make(chan isc.OnLedgerRequest),
		incomingRequests: events.NewEvent(func(handler interface{}, params ...interface{}) {
			handler.(func(_ isc.Request))(params[0].(isc.Request))
		}),
	}

	pool.gpa = mempoolgpa.New(
		pool.ReceiveRequests,
		pool.GetRequest,
		pool.log,
	)
	pool.netPeeringID = peering.PeeringIDFromBytes(
		hashing.HashDataBlake2b(chainID.Bytes(), []byte("mempool")).Bytes(),
	)
	attachID := net.Attach(&pool.netPeeringID, peering.PeerMessageReceiverMempool, func(recv *peering.PeerMessageIn) {
		if recv.MsgType != msgTypeMempool {
			pool.log.Warnf("Unexpected message, type=%v", recv.MsgType)
			return
		}
		msg, err := pool.gpa.UnmarshalMessage(recv.MsgData)
		if err != nil {
			pool.log.Warnf("cannot parse message: %v", err)
			return
		}
		// process the incoming message and send whatever is needed to other peers
		pool.sendNetworkMessages(pool.gpa.Message(msg))
	})

	go func() {
		<-pool.ctx.Done()
		net.Detach(attachID)
	}()

	go pool.addTimelockedRequestsToMempool()

	go pool.gpaTick()

	return pool
}

// TODO this must be called from chainMGR
func (m *mempool) SetPeers(committee []gpa.NodeID, accessNodes []gpa.NodeID) {
	m.gpa.SetPeers(committee, accessNodes)
}

func (m *mempool) sendNetworkMessages(outMsgs gpa.OutMessages) {
	sendMsg := func(msg gpa.Message) {
		msgData, err := msg.MarshalBinary()
		if err != nil {
			m.log.Warnf("Failed to send a message: %v", err)
			return
		}
		peerMsg := &peering.PeerMessageData{
			PeeringID:   m.netPeeringID,
			MsgReceiver: peering.PeerMessageReceiverChainCons,
			MsgType:     msgTypeMempool,
			MsgData:     msgData,
		}
		m.net.SendMsgByPubKey(m.peers[msg.Recipient()], peerMsg)
	}
	outMsgs.MustIterate(sendMsg)
}

func (m *mempool) attachToIncomingRequests(handler func(isc.Request)) *events.Closure {
	closure := events.NewClosure(handler)
	m.incomingRequests.Hook(closure)
	return closure
}

func shouldBeRemoved(req isc.Request, currentTime time.Time) bool {
	onLedgerReq, ok := req.(isc.OnLedgerRequest)
	if !ok {
		return false
	}

	// TODO Do not process anything with SDRUC for now
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
			if nextUnlock.IsZero() || unlockTime.Before(nextUnlock) {
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
				for id, req := range nextUnlockReqs {
					m.addToPoolNoLock(req)
					delete(timelockedRequests, id)
				}
				m.poolMutex.Unlock()
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

const gpaTickInterval = 100 * time.Millisecond

func (m *mempool) gpaTick() {
	ticker := time.NewTicker(gpaTickInterval)
	for {
		select {
		case t := <-ticker.C:
			go m.sendNetworkMessages(m.gpa.Input(t))
		case <-m.ctx.Done():
			return
		}
	}
}

// adds a request to the pool after doing some basic checks, returns whether it was added successfully
func (m *mempool) addToPoolNoLock(req isc.Request) bool {
	if shouldBeRemoved(req, time.Now()) {
		return false // if expired or shouldn't even be processed, don't add to mempool
	}
	// checking in the state if request is processed or already in mempool.
	reqid := req.ID()
	if _, ok := m.pool[reqid]; ok {
		return false
	}
	if m.hasBeenProcessed(reqid) {
		return false
	}
	m.pool[reqid] = req
	m.log.Debugf("IN MEMPOOL %s (+%d / -%d)", req.ID(), m.inPoolCounter, m.outPoolCounter)
	m.inPoolCounter++
	m.metrics.CountRequestIn(req)
	m.incomingRequests.Trigger(req)
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
		onledgerReq, ok := req.(isc.OnLedgerRequest)
		if !ok {
			// offledger
			go m.sendNetworkMessages(m.gpa.Input(req))
			ret[i] = m.addToPoolNoLock(req)
			continue
		}
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
		ret[i] = m.addToPoolNoLock(req)
	}
	return ret
}

func (m *mempool) removeFromPoolNoLock(reqID isc.RequestID) {
	m.outPoolCounter++
	delete(m.pool, reqID)
	m.log.Debugf("OUT MEMPOOL %s (+%d / -%d)", reqID, m.inPoolCounter, m.outPoolCounter)
	m.metrics.CountRequestOut()
	m.metrics.CountBlocksPerChain()
}

func (m *mempool) removeRequests(reqs ...isc.RequestID) {
	if len(reqs) == 0 {
		return
	}
	m.gpa.Input(mempoolgpa.RemovedFromMempool{
		RequestIDs: reqs,
	})
	m.poolMutex.Lock()
	defer m.poolMutex.Unlock()

	for _, rid := range reqs {
		if _, ok := m.pool[rid]; !ok {
			continue
		}
		m.removeFromPoolNoLock(rid)
	}
}

func (m *mempool) Empty() bool {
	m.poolMutex.RLock()
	defer m.poolMutex.RUnlock()
	return len(m.pool) == 0
}

const checkForRequestsInPoolInterval = 200 * time.Millisecond

// ConsensusProposalsAsync returns a list of requests to be sent as a batch proposal
func (m *mempool) ConsensusProposalsAsync(ctx context.Context, aliasOutput *isc.AliasOutputWithID) <-chan []*isc.RequestRef {
	// TODO handle reorgs (if possible, TBD)

	if aliasOutput.GetStateIndex() == 0 {
		m.lastSeenChainOutput = aliasOutput
	} else {
		// clean mempool from requests processed since lastSeenChainOutput until aliasOutput
		lastSeenStateIndex := uint32(0)
		if m.lastSeenChainOutput != nil {
			lastSeenStateIndex = m.lastSeenChainOutput.GetStateIndex()
		}
		if aliasOutput.GetStateIndex() < lastSeenStateIndex {
			panic(fmt.Sprintf("reorg happened, last seen: %s, received: %s", m.lastSeenChainOutput.String(), aliasOutput.String()))
		}
		processedReqs := m.getProcessedRequests(m.lastSeenChainOutput, aliasOutput)
		m.removeRequests(processedReqs...)
	}

	retChan := make(chan []*isc.RequestRef, 1)
	go func() {
		// wait for some time or until the pool is not empty
		for m.Empty() {
			time.Sleep(checkForRequestsInPoolInterval)
			if ctx.Err() != nil {
				close(retChan)
				return
			}
		}
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
		m.removeRequests(toRemove...)
	}()

	return retChan
}

func (m *mempool) getRequestsFromRefs(requestRefs []*isc.RequestRef) (requests []isc.Request, missingReqs map[hashing.HashValue]int) {
	m.poolMutex.RLock()
	defer m.poolMutex.RUnlock()

	requests = make([]isc.Request, len(requestRefs))
	missingReqs = make(map[hashing.HashValue]int)
	for i, ref := range requestRefs {
		req, ok := m.pool[ref.ID]
		if ok {
			requests[i] = req
		} else {
			missingReqs[ref.Hash] = i
		}
	}
	return requests, missingReqs
}

// ConsensusRequestsAsync return a list of requests to be processed
func (m *mempool) ConsensusRequestsAsync(ctx context.Context, requestRefs []*isc.RequestRef) <-chan []isc.Request {
	retChan := make(chan []isc.Request, 1)

	go func() {
		requests, missingReqs := m.getRequestsFromRefs(requestRefs)
		if len(missingReqs) == 0 {
			// we have all the requests
			retChan <- requests
			return
		}

		var missingRequestsChan chan isc.Request
		if len(missingReqs) > 0 {
			closure := m.attachToIncomingRequests(func(req isc.Request) {
				missingRequestsChan <- req
			})
			defer m.incomingRequests.Detach(closure)

			for _, idx := range missingReqs {
				missingRef := requestRefs[idx]
				go m.sendNetworkMessages(
					m.gpa.Input(missingRef),
				)
			}
		}

		for {
			select {
			case req := <-missingRequestsChan:
				reqHash := isc.RequestHash(req)
				idx, ok := missingReqs[reqHash]
				if !ok {
					continue // not the request we're looking for
				}
				requests[idx] = req
				delete(missingReqs, reqHash)
				if len(missingReqs) == 0 {
					// we have all the requests
					retChan <- requests
					return
				}
			case <-ctx.Done():
				return
			}
		}
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
