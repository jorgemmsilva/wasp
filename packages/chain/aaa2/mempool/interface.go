package mempool

import (
	"time"

	consGR "github.com/iotaledger/wasp/packages/chain/aaa2/cons/gr"
	"github.com/iotaledger/wasp/packages/isc"
)

type Mempool interface {
	consGR.Mempool
	ReceiveRequests(reqs ...isc.Request) []bool
	RemoveRequests(reqs ...isc.RequestID)
	HasRequest(id isc.RequestID) bool
	GetRequest(id isc.RequestID) isc.Request
	Info(currentTime time.Time) MempoolInfo
	Close()
}

// for testing (only for use in solo)
type SoloMempool interface {
	Mempool
	consGR.Mempool // TODO should this be unified with the Mempool interface above?
	WaitPoolEmpty(timeout ...time.Duration) bool
}

type MempoolInfo struct {
	TotalPool      int
	InPoolCounter  int
	OutPoolCounter int
}
