// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package cmt_log

import (
	"fmt"

	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
)

type inputConsensusOutputConfirmed struct {
	accountOutput *isc.AccountOutputWithID
	logIndex    LogIndex
}

func NewInputConsensusOutputConfirmed(accountOutput *isc.AccountOutputWithID, logIndex LogIndex) gpa.Input {
	return &inputConsensusOutputConfirmed{
		accountOutput: accountOutput,
		logIndex:    logIndex,
	}
}

func (inp *inputConsensusOutputConfirmed) String() string {
	return fmt.Sprintf("{cmtLog.inputConsensusOutputConfirmed, %v, li=%v}", inp.accountOutput, inp.logIndex)
}
