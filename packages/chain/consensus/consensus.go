package consensus

import (
	"time"

	"github.com/iotaledger/wasp/packages/coretypes/assert"

	"github.com/iotaledger/wasp/packages/hashing"

	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/runvm"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/wasp/packages/state"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/wasp/packages/chain"
	"go.uber.org/atomic"
)

type Consensus struct {
	isReady                    atomic.Bool
	chain                      chain.ChainCore
	committee                  chain.Committee
	mempool                    chain.Mempool
	nodeConn                   chain.NodeConnection
	vmRunner                   vm.VMRunner
	currentState               state.VirtualState
	stateOutput                *ledgerstate.AliasOutput
	stateTimestamp             time.Time
	acsSessionID               uint64
	consensusBatch             *BatchProposal
	consensusEntropy           hashing.HashValue
	iAmContributor             bool
	myContributionSeqNumber    uint16
	contributors               []uint16
	workflow                   workflowFlags
	delayBatchProposalUntil    time.Time
	delayRunVMUntil            time.Time
	resultTxEssence            *ledgerstate.TransactionEssence
	resultState                state.VirtualState
	resultSignatures           []*chain.SignedResultMsg
	finalTx                    *ledgerstate.Transaction
	postTxDeadline             time.Time
	pullInclusionStateDeadline time.Time
	lastTimerTick              atomic.Int64
	consensusInfoSnapshot      atomic.Value
	log                        *logger.Logger
	eventStateTransitionMsgCh  chan *chain.StateTransitionMsg
	eventSignedResultMsgCh     chan *chain.SignedResultMsg
	eventInclusionStateMsgCh   chan *chain.InclusionStateMsg
	eventACSMsgCh              chan *chain.AsynchronousCommonSubsetMsg
	eventVMResultMsgCh         chan *chain.VMResultMsg
	eventTimerMsgCh            chan chain.TimerTick
	closeCh                    chan struct{}
	assert                     assert.Assert
}

type workflowFlags struct {
	stateReceived                bool
	batchProposalSent            bool
	consensusBatchKnown          bool
	vmStarted                    bool
	vmResultSignedAndBroadcasted bool
	transactionFinalized         bool
	transactionPosted            bool
	transactionSeen              bool
	finished                     bool
}

var _ chain.Consensus = &Consensus{}

func New(chainCore chain.ChainCore, mempool chain.Mempool, committee chain.Committee, nodeConn chain.NodeConnection) *Consensus {
	log := chainCore.Log().Named("c")
	ret := &Consensus{
		chain:                     chainCore,
		committee:                 committee,
		mempool:                   mempool,
		nodeConn:                  nodeConn,
		vmRunner:                  runvm.NewVMRunner(),
		resultSignatures:          make([]*chain.SignedResultMsg, committee.Size()),
		log:                       log,
		eventStateTransitionMsgCh: make(chan *chain.StateTransitionMsg),
		eventSignedResultMsgCh:    make(chan *chain.SignedResultMsg),
		eventInclusionStateMsgCh:  make(chan *chain.InclusionStateMsg),
		eventACSMsgCh:             make(chan *chain.AsynchronousCommonSubsetMsg),
		eventVMResultMsgCh:        make(chan *chain.VMResultMsg),
		eventTimerMsgCh:           make(chan chain.TimerTick),
		closeCh:                   make(chan struct{}),
		assert:                    assert.NewAssert(log),
	}
	ret.refreshConsensusInfo()
	go ret.recvLoop()
	return ret
}

func (c *Consensus) IsReady() bool {
	return c.isReady.Load()
}

func (c *Consensus) Close() {
	close(c.closeCh)
}

func (c *Consensus) recvLoop() {
	// wait at startup
	for !c.committee.IsReady() {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-c.closeCh:
			return
		}
	}
	c.log.Debugf("consensus object is ready")
	c.isReady.Store(true)
	for {
		select {
		case msg, ok := <-c.eventStateTransitionMsgCh:
			if ok {
				c.eventStateTransitionMsg(msg)
			}
		case msg, ok := <-c.eventSignedResultMsgCh:
			if ok {
				c.eventSignedResult(msg)
			}
		case msg, ok := <-c.eventInclusionStateMsgCh:
			if ok {
				c.eventInclusionState(msg)
			}
		case msg, ok := <-c.eventACSMsgCh:
			if ok {
				c.eventAsynchronousCommonSubset(msg)
			}
		case msg, ok := <-c.eventVMResultMsgCh:
			if ok {
				c.eventVMResultMsg(msg)
			}
		case msg, ok := <-c.eventTimerMsgCh:
			if ok {
				c.eventTimerMsg(msg)
			}
		case <-c.closeCh:
			return
		}
	}
}

func (c *Consensus) refreshConsensusInfo() {
	index := uint32(0)
	if c.currentState != nil {
		index = c.currentState.BlockIndex()
	}
	c.consensusInfoSnapshot.Store(&chain.ConsensusInfo{
		StateIndex: index,
		Mempool:    c.mempool.Stats(),
		TimerTick:  int(c.lastTimerTick.Load()),
	})
}

func (c *Consensus) GetStatusSnapshot() *chain.ConsensusInfo {
	ret := c.consensusInfoSnapshot.Load()
	if ret == nil {
		return nil
	}
	return ret.(*chain.ConsensusInfo)
}
