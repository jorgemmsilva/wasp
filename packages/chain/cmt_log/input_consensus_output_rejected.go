// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package cmt_log

import (
	"fmt"

	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
)

type inputConsensusOutputRejected struct {
	accountOutput *isc.AccountOutputWithID
	logIndex    LogIndex
}

func NewInputConsensusOutputRejected(accountOutput *isc.AccountOutputWithID, logIndex LogIndex) gpa.Input {
	return &inputConsensusOutputRejected{
		accountOutput: accountOutput,
		logIndex:    logIndex,
	}
}

func (inp *inputConsensusOutputRejected) String() string {
	return fmt.Sprintf("{cmtLog.inputConsensusOutputRejected, %v, li=%v}", inp.accountOutput, inp.logIndex)
}
