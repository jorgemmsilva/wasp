package mempoolgpa

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
)

const (
	shareReqNNodes   = 1
	shareReqInterval = 1 * time.Second
)

const (
	askMissingReqNNodes   = 2
	askMissingReqInterval = 300 * time.Millisecond
)

type Impl struct {
	receiveRequests         func(reqs ...isc.Request) []bool
	getRequest              func(id isc.RequestID) isc.Request
	hasRequestBeenProcessed func(id isc.RequestID) bool
	committeeNodes          []gpa.NodeID
	accessNodes             []gpa.NodeID
	peersMutex              sync.RWMutex
	log                     *logger.Logger
}

func New(
	receiveRequests func(reqs ...isc.Request) []bool,
	getRequest func(id isc.RequestID) isc.Request,
	hasRequestBeenProcessed func(id isc.RequestID) bool,
	log *logger.Logger,
) *Impl {
	return &Impl{
		receiveRequests:         receiveRequests,
		getRequest:              getRequest,
		hasRequestBeenProcessed: hasRequestBeenProcessed,
		log:                     log,
	}
}

func (m *Impl) SetPeers(committeeNodes, accessNodes []gpa.NodeID) {
	m.peersMutex.Lock()
	defer m.peersMutex.Unlock()
	m.committeeNodes = committeeNodes
	m.accessNodes = accessNodes
}

func (m *Impl) NewShareRequestMessages(req isc.Request, receivedFrom gpa.NodeID) *MempoolMessages {
	msgs := gpa.NoMessages()
	// share to committee and access nodes
	allNodes := m.committeeNodes
	allNodes = append(allNodes, m.accessNodes...)
	for _, nodeID := range allNodes {
		if nodeID != receivedFrom {
			msgs.Add(newMsgShareRequest(req, true, nodeID))
		}
	}
	// TODO keep track of who has the request
	return NewStaggeredMessages(
		msgs,
		shareReqNNodes,
		shareReqInterval,
		func() bool {
			reqHasLeftTheMempool := m.getRequest(req.ID()) == nil
			return reqHasLeftTheMempool
		})
}

func (m *Impl) Input(input gpa.Input) *MempoolMessages {
	switch inp := input.(type) {
	case isc.Request:
		return m.NewShareRequestMessages(inp, gpa.NodeID(""))
	case *isc.RequestRef:
		msgs := gpa.NoMessages()
		for _, nodeID := range m.committeeNodes {
			msgs.Add(newMsgMissingRequest(inp, nodeID))
		}
		return NewStaggeredMessages(
			msgs,
			askMissingReqNNodes,
			askMissingReqInterval,
			func() bool {
				return m.hasRequestBeenProcessed(inp.ID)
			},
		)
	default:
		m.log.Warnf("unexpected input %T: %+v", input, input)
	}
	return NoMessages()
}

// Message handles INCOMMING messages (from other nodes)
func (m *Impl) Message(msg gpa.Message) *MempoolMessages {
	switch message := msg.(type) {
	case *msgShareRequest:
		return m.receiveMsgShareRequests(message)
	case *msgMissingRequest:
		return m.receiveMsgMissingRequest(message)
	default:
		m.log.Warnf("unexpected message %T: %+v", msg, msg)
	}
	return NoMessages()
}

func (m *Impl) receiveMsgShareRequests(msg *msgShareRequest) *MempoolMessages {
	res := m.receiveRequests(msg.req)
	if len(res) != 1 || !res[0] {
		// message was rejected by the mempool
		return NoMessages()
	}
	if !msg.shouldPropagate {
		return NoMessages()
	}
	return m.NewShareRequestMessages(msg.req, msg.Sender())
}

// sender of the message is missing a request
func (m *Impl) receiveMsgMissingRequest(input *msgMissingRequest) *MempoolMessages {
	req := m.getRequest(input.ref.ID)
	if req == nil {
		return NoMessages()
	}
	if !input.ref.IsFor(req) {
		m.log.Warnf("mismatch between requested requestRef and request in mempool. refHash: %s request:%s", input.ref.Hash.Hex(), req.String())
		return NoMessages()
	}
	return SingleMessage(
		newMsgShareRequest(req, false, input.Sender()),
	)
}

func (m *Impl) UnmarshalMessage(data []byte) (msg gpa.Message, err error) {
	switch data[0] {
	case msgTypeMissingRequest:
		msg = &msgMissingRequest{}
	case msgTypeShareRequest:
		msg = &msgShareRequest{}
	default:
		return nil, fmt.Errorf("unknown message type %b", data[0])
	}
	err = msg.UnmarshalBinary(data)
	if err != nil {
		return nil, err
	}
	return msg, nil
}
