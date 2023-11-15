// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package cmt_log

import (
	"fmt"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
)

type inputConsensusOutputDone struct {
	logIndex           LogIndex
	proposedBaseAO     iotago.OutputID   // Proposed BaseAO
	baseAnchorOutputID iotago.OutputID   // Decided BaseAO
	nextAnchorOutput   *isc.ChainOutputs // And the next one.
}

// This message is internal one, but should be sent by other components (e.g. consensus or the chain).
func NewInputConsensusOutputDone(
	logIndex LogIndex,
	proposedBaseAO iotago.OutputID,
	baseAnchorOutputID iotago.OutputID,
	nextAnchorOutput *isc.ChainOutputs,
) gpa.Input {
	return &inputConsensusOutputDone{
		logIndex:           logIndex,
		proposedBaseAO:     proposedBaseAO,
		baseAnchorOutputID: baseAnchorOutputID,
		nextAnchorOutput:   nextAnchorOutput,
	}
}

func (inp *inputConsensusOutputDone) String() string {
	return fmt.Sprintf(
		"{cmtLog.inputConsensusOutputDone, logIndex=%v, proposedBaseAO=%v, baseAnchorOutputID=%v, nextAnchorOutput=%v}",
		inp.logIndex, inp.proposedBaseAO.ToHex(), inp.baseAnchorOutputID.ToHex(), inp.nextAnchorOutput,
	)
}
