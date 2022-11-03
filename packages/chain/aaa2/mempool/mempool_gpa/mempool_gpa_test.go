package mempool_gpa

import (
	"testing"

	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
)

func TestMempoolGpa(t *testing.T) {
	sendMessages := func(outMsgs gpa.OutMessages) {
	}
	receiveRequests := func(reqs ...isc.Request) []bool {
	}
	getRequest := func(id isc.RequestID) isc.Request {
	}
	gpa := New(
		sendMessages,
		receiveRequests,
		getRequest,
		testlogger.NewLogger(t),
	)
}
