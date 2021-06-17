package testchain

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/wasp/packages/chain"
	"github.com/iotaledger/wasp/packages/coretypes"
	"github.com/iotaledger/wasp/packages/coretypes/chainid"
	"github.com/iotaledger/wasp/packages/coretypes/coreutil"
	"github.com/iotaledger/wasp/packages/coretypes/request"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/vm/processors"
	"go.uber.org/atomic"
)

type MockedChainCore struct {
	T                       *testing.T
	chainID                 chainid.ChainID
	processors              *processors.ProcessorCache
	eventStateTransition    *events.Event
	eventRequestProcessed   *events.Event
	eventStateSynced        *events.Event
	onGlobalStateSync       func() coreutil.ChainStateSync
	onGetStateReader        func() state.OptimisticStateReader
	onEventStateTransition  func(data *chain.ChainTransitionEventData)
	onEventRequestProcessed func(id coretypes.RequestID)
	onEventStateSynced      func(id ledgerstate.OutputID, blockIndex uint32)
	onReceiveMessage        func(i interface{})
	onSync                  func(out ledgerstate.OutputID, blockIndex uint32) //nolint:structcheck,unused
	log                     *logger.Logger
}

func NewMockedChainCore(t *testing.T, chainID chainid.ChainID, log *logger.Logger) *MockedChainCore {
	ret := &MockedChainCore{
		T:          t,
		chainID:    chainID,
		processors: processors.MustNew(),
		log:        log,
		eventStateTransition: events.NewEvent(func(handler interface{}, params ...interface{}) {
			handler.(func(_ *chain.ChainTransitionEventData))(params[0].(*chain.ChainTransitionEventData))
		}),
		eventRequestProcessed: events.NewEvent(func(handler interface{}, params ...interface{}) {
			handler.(func(_ coretypes.RequestID))(params[0].(coretypes.RequestID))
		}),
		eventStateSynced: events.NewEvent(func(handler interface{}, params ...interface{}) {
			handler.(func(outputID ledgerstate.OutputID, blockIndex uint32))(params[0].(ledgerstate.OutputID), params[1].(uint32))
		}),
		onEventRequestProcessed: func(id coretypes.RequestID) {
			log.Infof("onEventRequestProcessed: %s", id)
		},
		onEventStateSynced: func(outputID ledgerstate.OutputID, blockIndex uint32) {
			chain.LogSyncedEvent(outputID, blockIndex, log)
		},
	}
	ret.onEventStateTransition = func(msg *chain.ChainTransitionEventData) {
		chain.LogStateTransition(msg, nil, log)
	}
	ret.eventStateTransition.Attach(events.NewClosure(func(data *chain.ChainTransitionEventData) {
		ret.onEventStateTransition(data)
	}))
	ret.eventRequestProcessed.Attach(events.NewClosure(func(id coretypes.RequestID) {
		ret.onEventRequestProcessed(id)
	}))
	ret.eventStateSynced.Attach(events.NewClosure(func(outid ledgerstate.OutputID, blockIndex uint32) {
		ret.onEventStateSynced(outid, blockIndex)
	}))
	return ret
}

func (m *MockedChainCore) Log() *logger.Logger {
	return m.log
}

func (m *MockedChainCore) ID() *chainid.ChainID {
	return &m.chainID
}

func (c *MockedChainCore) GlobalStateSync() coreutil.ChainStateSync {
	return c.onGlobalStateSync()
}

func (m *MockedChainCore) GetStateReader() state.OptimisticStateReader {
	return m.onGetStateReader()
}

func (m *MockedChainCore) GetCommitteeInfo() *chain.CommitteeInfo {
	panic("implement me")
}

func (m *MockedChainCore) ReceiveMessage(i interface{}) {
	m.onReceiveMessage(i)
}

func (m *MockedChainCore) Events() chain.ChainEvents {
	return m
}

func (m *MockedChainCore) Processors() *processors.ProcessorCache {
	return m.processors
}

func (m *MockedChainCore) RequestProcessed() *events.Event {
	return m.eventRequestProcessed
}

func (m *MockedChainCore) ChainTransition() *events.Event {
	return m.eventStateTransition
}

func (m *MockedChainCore) StateSynced() *events.Event {
	return m.eventStateSynced
}

func (m *MockedChainCore) OnStateTransition(f func(data *chain.ChainTransitionEventData)) {
	m.onEventStateTransition = f
}

func (m *MockedChainCore) OnRequestProcessed(f func(id coretypes.RequestID)) {
	m.onEventRequestProcessed = f
}

func (m *MockedChainCore) OnReceiveMessage(f func(i interface{})) {
	m.onReceiveMessage = f
}

func (m *MockedChainCore) OnStateSynced(f func(out ledgerstate.OutputID, blockIndex uint32)) {
	m.onEventStateSynced = f
}

func (m *MockedChainCore) OnGetStateReader(f func() state.OptimisticStateReader) {
	m.onGetStateReader = f
}

func (m *MockedChainCore) OnGlobalStateSync(f func() coreutil.ChainStateSync) {
	m.onGlobalStateSync = f
}

func (m *MockedChainCore) GlobalSolidIndex() *atomic.Uint32 {
	return nil
}

func (m *MockedChainCore) ReceiveOffLedgerRequest(req *request.RequestOffLedger) {
}
