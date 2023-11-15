// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package cmt_log

import (
	"fmt"

	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
)

type inputConsensusOutputConfirmed struct {
	anchorOutput *isc.ChainOutputs
	logIndex     LogIndex
}

func NewInputConsensusOutputConfirmed(anchorOutput *isc.ChainOutputs, logIndex LogIndex) gpa.Input {
	return &inputConsensusOutputConfirmed{
		anchorOutput: anchorOutput,
		logIndex:     logIndex,
	}
}

func (inp *inputConsensusOutputConfirmed) String() string {
	return fmt.Sprintf("{cmtLog.inputConsensusOutputConfirmed, %v, li=%v}", inp.anchorOutput, inp.logIndex)
}
