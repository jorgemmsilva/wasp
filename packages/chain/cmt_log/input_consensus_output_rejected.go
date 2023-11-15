// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package cmt_log

import (
	"fmt"

	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
)

type inputConsensusOutputRejected struct {
	anchorOutput *isc.ChainOutputs
	logIndex     LogIndex
}

func NewInputConsensusOutputRejected(anchorOutput *isc.ChainOutputs, logIndex LogIndex) gpa.Input {
	return &inputConsensusOutputRejected{
		anchorOutput: anchorOutput,
		logIndex:     logIndex,
	}
}

func (inp *inputConsensusOutputRejected) String() string {
	return fmt.Sprintf("{cmtLog.inputConsensusOutputRejected, %v, li=%v}", inp.anchorOutput, inp.logIndex)
}
