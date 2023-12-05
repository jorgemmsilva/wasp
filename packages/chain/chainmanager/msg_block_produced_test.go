package chainmanager

import (
	"testing"

	"github.com/iotaledger/iota.go/v4/tpkg"
	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

func TestMsgBlockProducedSerialization(t *testing.T) {
	msg := &msgBlockProduced{
		gpa.BasicMessage{},
		tpkg.RandSignedTransaction(testutil.L1API),
		state.RandomBlock(),
	}

	rwutil.ReadWriteTest(t, msg, new(msgBlockProduced))
}
