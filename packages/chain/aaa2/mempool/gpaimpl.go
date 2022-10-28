package mempool

import (
	"fmt"

	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/peering"
)

func (m *mempool) AsGPA() gpa.GPA {
	return m.asGPA
}

func (m *mempool) Input(input gpa.Input) gpa.OutMessages {
	switch input := input.(type) {
	case *msgShareRequest:
		return m.handleInputShareRequests(input)
	case *msgMissingRequest:
		return m.handleInputMissingRequest(input)
	}
	panic(fmt.Errorf("unexpected input %T: %+v", input, input))
}

func (m *mempool) ReceivePeerMsg(recv *peering.PeerMessageIn) {
	msg, err := m.UnmarshalMessage(recv.MsgData)
	if err != nil {
		m.log.Warnf("cannot parse message: %v", err)
		return
	}
	// TODO call msg func
	// m.sendMessages(m.Input(msg))
}

func (m *mempool) handleInputShareRequests(input *msgShareRequest) gpa.OutMessages {
	m.ReceiveRequests(input.req)
	// TODO send messages to other peers
	return gpa.NoMessages().Add(input)
}

func (m *mempool) handleInputMissingRequest(input *msgMissingRequest) gpa.OutMessages {
	req := m.GetRequest(input.ref.ID)
	if req == nil {
		return gpa.NoMessages()
	}
	if !input.ref.IsFor(req) {
		m.log.Warnf("mismatch between requested requestRef and request in mempool. refHash: %s request:%s", input.ref.Hash.Hex(), req.String())
		return gpa.NoMessages()
	}
	msg := newMsgShareRequest(req)

	// TODO  why can't I set the receipient?
	// msg.SetReceipient(input.Sender())

	return gpa.NoMessages().Add(msg)
}

// HANDLE INCOMMING MESSAGE (from other nodes)
func (m *mempool) Message(msg gpa.Message) gpa.OutMessages {
	panic("unimplemented")
}

const (
	msgTypeMempool byte = iota
)

func (m *mempool) sendMessages(outMsgs gpa.OutMessages) {
	outMsgs.MustIterate(func(msg gpa.Message) {
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
	})
}

func (m *mempool) UnmarshalMessage(data []byte) (msg gpa.Message, err error) {
	switch data[0] {
	case msgTypeMissingRequest:
		msg = &msgMissingRequest{}
		err = msg.UnmarshalBinary(data)
	case msgTypeShareRequest:
		msg = &msgShareRequest{}
		err = msg.UnmarshalBinary(data)
	default:
		return nil, fmt.Errorf("unknown message type %b", data[0])
	}
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// ------------------------------------------------------

// TODO what do I do with the output?
// resp: the output should be some requests to be consumed by the mempool...
// maybe output can be a callback.

type Output struct {
	pool *mempool
}

func (m *mempool) Output() gpa.Output {
	// use output as a "buffer" of incoming requests
	panic("unimplemented")
}

func (m *mempool) StatusString() string {
	return fmt.Sprintf("?")
}

// -------------------------------------------------------

func (m *mempool) sendMissingRequestMsg(missingRef *isc.RequestRef) {
	msg := newMsgMissingRequest(missingRef)
	// TODO set the receipient in the constructor

	// ask 1 peer at a time
	// TODO decide when to retry/ask again
	// peers := m.peers.GetRandomOtherPeers(1)
	// m.net.SendMsgByPubKey(peers[0], msg)
	m.sendMessages(gpa.NoMessages().Add(msg))
}

func (m *mempool) sendShareRequestMsg(req isc.Request) {
	msg := newMsgShareRequest(req)
	// TODO set the receipient in the constructor
	m.sendMessages(gpa.NoMessages().Add(msg))
}
