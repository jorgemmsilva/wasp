package mempool_gpa

import (
	"fmt"

	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
)

// TODO how to keep track of the peers

type mempoolGPA struct {
	receiveRequests func(reqs ...isc.Request) []bool
	getRequest      func(id isc.RequestID) isc.Request
	log             *logger.Logger
}

func New(
	sendMessages func(outMsgs gpa.OutMessages),
	receiveRequests func(reqs ...isc.Request) []bool,
	getRequest func(id isc.RequestID) isc.Request,
	log *logger.Logger,
) gpa.GPA {
	return &mempoolGPA{
		receiveRequests: receiveRequests,
		getRequest:      getRequest,
		log:             log,
	}
}

func (m *mempoolGPA) Input(input gpa.Input) gpa.OutMessages {
	msgs := gpa.NoMessages()
	switch inp := input.(type) {
	case isc.Request:
		// TODO add many peers
		msgs.Add(newMsgShareRequest(inp, true, ""))
	case *isc.RequestRef:
		// TODO add peers
		// TODO should we know
		msgs.Add(newMsgMissingRequest(inp, ""))
	default:
		m.log.Warnf("unexpected input %T: %+v", input, input)
	}
	return msgs
}

// HANDLE INCOMMING MESSAGE (from other nodes)
func (m *mempoolGPA) Message(msg gpa.Message) gpa.OutMessages {
	switch message := msg.(type) {
	case *msgShareRequest:
		return m.handleShareRequests(message)
	case *msgMissingRequest:
		return m.handleMissingRequest(message)
	default:
		m.log.Warnf("unexpected message %T: %+v", msg, msg)
	}
	return gpa.NoMessages()
}

func (m *mempoolGPA) handleShareRequests(input *msgShareRequest) gpa.OutMessages {
	m.receiveRequests(input.req)
	messages := gpa.NoMessages()
	if input.shouldPropagate {
		// TODO send messages to other peers
		nodeID := gpa.NodeID("")
		messages.Add(newMsgShareRequest(input.req, true, nodeID))
	}
	return messages.Add(input)
}

func (m *mempoolGPA) handleMissingRequest(input *msgMissingRequest) gpa.OutMessages {
	req := m.getRequest(input.ref.ID)
	if req == nil {
		return gpa.NoMessages()
	}
	if !input.ref.IsFor(req) {
		m.log.Warnf("mismatch between requested requestRef and request in mempool. refHash: %s request:%s", input.ref.Hash.Hex(), req.String())
		return gpa.NoMessages()
	}
	msg := newMsgShareRequest(req, false, input.Sender())
	return gpa.NoMessages().Add(msg)
}

func (m *mempoolGPA) UnmarshalMessage(data []byte) (msg gpa.Message, err error) {
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

// ------------------------------------------------------

// Output is unused
func (m *mempoolGPA) Output() gpa.Output {
	panic("unimplemented")
}

func (m *mempoolGPA) StatusString() string {
	panic("unimplemented")
}
