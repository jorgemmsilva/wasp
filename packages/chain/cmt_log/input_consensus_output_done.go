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
	logIndex          LogIndex
	proposedBaseAO    iotago.OutputID        // Proposed BaseAO
	baseAccountOutputID iotago.OutputID        // Decided BaseAO
	nextAccountOutput   *isc.AccountOutputWithID // And the next one.
}

// This message is internal one, but should be sent by other components (e.g. consensus or the chain).
func NewInputConsensusOutputDone(
	logIndex LogIndex,
	proposedBaseAO iotago.OutputID,
	baseAccountOutputID iotago.OutputID,
	nextAccountOutput *isc.AccountOutputWithID,
) gpa.Input {
	return &inputConsensusOutputDone{
		logIndex:          logIndex,
		proposedBaseAO:    proposedBaseAO,
		baseAccountOutputID: baseAccountOutputID,
		nextAccountOutput:   nextAccountOutput,
	}
}

func (inp *inputConsensusOutputDone) String() string {
	return fmt.Sprintf(
		"{cmtLog.inputConsensusOutputDone, logIndex=%v, proposedBaseAO=%v, baseAccountOutputID=%v, nextAccountOutput=%v}",
		inp.logIndex, inp.proposedBaseAO.ToHex(), inp.baseAccountOutputID.ToHex(), inp.nextAccountOutput,
	)
}
