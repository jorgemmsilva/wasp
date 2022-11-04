package mempool_gpa

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
)

func TestMempoolGpa(t *testing.T) {
	kp := cryptolib.NewKeyPair()
	requestA := isc.NewOffLedgerRequest(isc.RandomChainID(), isc.Hn("foo"), isc.Hn("bar"), nil, 0).Sign(kp)
	// requestB := isc.NewOffLedgerRequest(isc.RandomChainID(), isc.Hn("foo"), isc.Hn("bar"), nil, 1).Sign(kp)

	sendMessages := func(outMsgs gpa.OutMessages) {}
	receiveRequests := func(reqs ...isc.Request) []bool {
		return nil
	}
	getRequest := func(id isc.RequestID) isc.Request {
		// only requestA is in mempool
		if id == requestA.ID() {
			return requestA
		}
		return nil
	}
	poolGPA := New(
		sendMessages,
		receiveRequests,
		getRequest,
		testlogger.NewLogger(t),
	)
	t.Run("Input", func(t *testing.T) {
		msgs := poolGPA.Input(requestA)
		require.Len(t, msgs.AsArray(), 1) // TODO should depend on the number of peers
		shareReqMsg, ok := msgs.AsArray()[0].(*msgShareRequest)
		require.True(t, ok)
		require.Equal(t, shareReqMsg.req, requestA)
		require.True(t, shareReqMsg.shouldPropagate)
		require.Equal(t, shareReqMsg.Recipient(), gpa.NodeID("")) // TODO how should the peer be set?

		ref := isc.RequestRefsFromRequests([]isc.Request{requestA})[0]
		msgs = poolGPA.Input(ref)
		require.Len(t, msgs.AsArray(), 1) // TODO should depend on the number of peers
		missingReqMsg, ok := msgs.AsArray()[0].(*msgMissingRequest)
		require.True(t, ok)
		require.Equal(t, missingReqMsg.ref, ref)
		require.Equal(t, shareReqMsg.Recipient(), gpa.NodeID("")) // TODO how should the peer be set?
	})

	t.Run("Messages", func(t *testing.T) {})
}
