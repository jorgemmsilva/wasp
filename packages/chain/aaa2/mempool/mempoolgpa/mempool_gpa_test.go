package mempoolgpa

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
)

func TestMempoolGpa(t *testing.T) {
	kp := cryptolib.NewKeyPair()
	requestA := isc.NewOffLedgerRequest(isc.RandomChainID(), isc.Hn("foo"), isc.Hn("bar"), nil, 0).Sign(kp)
	requestB := isc.NewOffLedgerRequest(isc.RandomChainID(), isc.Hn("foo"), isc.Hn("bar"), nil, 1).Sign(kp)

	reqsInPool := make(map[isc.RequestID]isc.Request)
	reqsInPool[requestA.ID()] = requestA
	receiveRequests := func(reqs ...isc.Request) []bool {
		// mock implementation, only receives 1 request
		if _, ok := reqsInPool[reqs[0].ID()]; ok {
			return []bool{false}
		}
		reqsInPool[reqs[0].ID()] = reqs[0]
		return []bool{true}
	}
	getRequest := func(id isc.RequestID) isc.Request {
		// only requestA is in mempool
		if id == requestA.ID() {
			return requestA
		}
		return nil
	}
	hasRequestBeenProcessed := func(id isc.RequestID) bool {
		// requestB has already been processed
		return id == requestB.ID()
	}
	poolGPA := New(
		receiveRequests,
		getRequest,
		hasRequestBeenProcessed,
		testlogger.NewLogger(t),
	)
	committeePeers := []gpa.NodeID{gpa.NodeID("committeNode1"), gpa.NodeID("committeNode2")}
	accessNodePeers := []gpa.NodeID{gpa.NodeID("accessNode1")}
	poolGPA.SetPeers(
		committeePeers,
		accessNodePeers,
	)

	t.Run("Input request", func(t *testing.T) {
		msgs := poolGPA.Input(requestA)
		require.Len(t, msgs.AsArray(), 3) // number of total peers
		require.True(t, msgs.Staggered)

		for _, m := range msgs.AsArray() {
			shareReqMsg, ok := m.(*msgShareRequest)
			require.True(t, ok)
			require.Equal(t, shareReqMsg.req, requestA)
			require.True(t, shareReqMsg.shouldPropagate)
			require.True(t,
				slices.Contains(committeePeers, shareReqMsg.Recipient()) ||
					slices.Contains(accessNodePeers, shareReqMsg.Recipient()),
			)
		}
	})

	t.Run("Input ref", func(t *testing.T) {
		ref := isc.RequestRefsFromRequests([]isc.Request{requestA})[0]
		msgs := poolGPA.Input(ref)
		require.Len(t, msgs.AsArray(), 2) // number of committee nodes
		require.True(t, msgs.Staggered)
		for _, m := range msgs.AsArray() {
			missingReqMsg, ok := m.(*msgMissingRequest)
			require.True(t, ok)
			require.Equal(t, missingReqMsg.ref, ref)
			require.Contains(t, committeePeers, missingReqMsg.Recipient())
		}
		require.False(t, msgs.ShouldStopSending())

		// should stopSending must return true if the request has already been processed (requestB)
		ref = isc.RequestRefsFromRequests([]isc.Request{requestB})[0]
		msgs = poolGPA.Input(ref)
		require.True(t, msgs.ShouldStopSending())
	})

	t.Run("Message receive shared request", func(t *testing.T) {
		// when receiving request A, since its already in the mempool, no message should be sent
		incomingMsg := newMsgShareRequest(requestA, true, gpa.NodeID(""))
		msgs := poolGPA.Message(incomingMsg)
		require.Len(t, msgs.AsArray(), 0)

		// when receiving request B with shouldPropagate=false, no messages should be sent, but the request should be received
		require.Len(t, reqsInPool, 1)
		incomingMsg = newMsgShareRequest(requestB, false, gpa.NodeID(""))
		msgs = poolGPA.Message(incomingMsg)
		require.Len(t, msgs.AsArray(), 0)
		require.Len(t, reqsInPool, 2)
		require.Equal(t, reqsInPool[requestB.ID()], requestB)

		// clear B so it can be received again
		delete(reqsInPool, requestB.ID())

		// receiving request with shouldPropagate=true
		incomingMsg = newMsgShareRequest(requestB, true, gpa.NodeID(""))
		incomingMsg.SetSender(gpa.NodeID("committeNode1"))
		msgs = poolGPA.Message(incomingMsg)
		require.Len(t, reqsInPool, 2)
		require.Equal(t, reqsInPool[requestB.ID()], requestB)
		require.Len(t, msgs.AsArray(), 2) // total number of peers - "committeNode1"
		for _, m := range msgs.AsArray() {
			shareReqMsg, ok := m.(*msgShareRequest)
			require.True(t, ok)
			require.Equal(t, shareReqMsg.req, requestB)
			require.True(t, shareReqMsg.shouldPropagate)
			require.True(t,
				slices.Contains(committeePeers, shareReqMsg.Recipient()) ||
					slices.Contains(accessNodePeers, shareReqMsg.Recipient()),
			)
		}
	})

	t.Run("Message receive asking for missing request", func(t *testing.T) {
		ref := isc.RequestRefsFromRequests([]isc.Request{requestA})[0]
		sender := gpa.NodeID("someID")
		incomingMsg := newMsgMissingRequest(ref, gpa.NodeID(""))
		incomingMsg.SetSender(sender)
		msgs := poolGPA.Message(incomingMsg)
		require.Len(t, msgs.AsArray(), 1)
		require.False(t, msgs.Staggered)
		m, ok := msgs.AsArray()[0].(*msgShareRequest)
		require.True(t, ok)
		require.Equal(t, sender, m.Recipient())
		require.Equal(t, m.req, requestA)
		require.False(t, m.shouldPropagate)

		// when asking for requestB, the resulst must be empty because it is not in the pool
		ref = isc.RequestRefsFromRequests([]isc.Request{requestB})[0]
		incomingMsg = newMsgMissingRequest(ref, gpa.NodeID(""))
		incomingMsg.SetSender(sender)
		msgs = poolGPA.Message(incomingMsg)
		require.Len(t, msgs.AsArray(), 0)
	})
}
