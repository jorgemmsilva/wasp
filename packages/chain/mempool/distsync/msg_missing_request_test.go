// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package distsync

import (
	"testing"

	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/util/rwutil"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

func TestMsgMissingRequestSerialization(t *testing.T) {
	senderKP := cryptolib.NewKeyPair()
	m := isc.NewMessage(governance.Contract.Hname(), governance.FuncAddCandidateNode.Hname(), nil)
	gasBudget := gas.LimitsDefault.MaxGasPerRequest
	req := isc.NewOffLedgerRequest(isc.RandomChainID(), m, 0, gasBudget).Sign(senderKP)
	msg := &msgMissingRequest{
		gpa.BasicMessage{},
		isc.RequestRefFromRequest(req),
	}
	rwutil.ReadWriteTest(t, msg, new(msgMissingRequest))
}
