// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package cons

import (
	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/state"
)

type SyncSM interface {
	//
	// State proposal.
	ProposedBaseAnchorOutputReceived(baseAnchorOutput *isc.AnchorOutputWithID) gpa.OutMessages
	StateProposalConfirmedByStateMgr() gpa.OutMessages
	//
	// Decided state.
	DecidedVirtualStateNeeded(decidedBaseAnchorOutput *isc.AnchorOutputWithID) gpa.OutMessages
	DecidedVirtualStateReceived(chainState state.State) gpa.OutMessages
	//
	// Save the block.
	BlockProduced(producedBlock state.StateDraft) gpa.OutMessages
	BlockSaved(savedBlock state.Block) gpa.OutMessages
	//
	// Supporting stuff.
	String() string
}

type syncSMImpl struct {
	//
	// Query for a proposal.
	proposedBaseAnchorOutput         *isc.AnchorOutputWithID
	stateProposalQueryInputsReadyCB func(baseAnchorOutput *isc.AnchorOutputWithID) gpa.OutMessages
	stateProposalReceived           bool
	stateProposalReceivedCB         func(proposedAnchorOutput *isc.AnchorOutputWithID) gpa.OutMessages
	//
	// Query for a decided Virtual State.
	decidedBaseAnchorOutput         *isc.AnchorOutputWithID
	decidedStateQueryInputsReadyCB func(decidedBaseAnchorOutput *isc.AnchorOutputWithID) gpa.OutMessages
	decidedStateReceived           bool
	decidedStateReceivedCB         func(chainState state.State) gpa.OutMessages
	//
	// Save the produced block.
	producedBlock                  state.StateDraft // In the case of rotation the block will be nil.
	producedBlockReceived          bool
	saveProducedBlockInputsReadyCB func(producedBlock state.StateDraft) gpa.OutMessages
	saveProducedBlockDone          bool
	saveProducedBlockDoneCB        func(savedBlock state.Block) gpa.OutMessages
}

func NewSyncSM(
	stateProposalQueryInputsReadyCB func(baseAnchorOutput *isc.AnchorOutputWithID) gpa.OutMessages,
	stateProposalReceivedCB func(proposedAnchorOutput *isc.AnchorOutputWithID) gpa.OutMessages,
	decidedStateQueryInputsReadyCB func(decidedBaseAnchorOutput *isc.AnchorOutputWithID) gpa.OutMessages,
	decidedStateReceivedCB func(chainState state.State) gpa.OutMessages,
	saveProducedBlockInputsReadyCB func(producedBlock state.StateDraft) gpa.OutMessages,
	saveProducedBlockDoneCB func(savedBlock state.Block) gpa.OutMessages,
) SyncSM {
	return &syncSMImpl{
		stateProposalQueryInputsReadyCB: stateProposalQueryInputsReadyCB,
		stateProposalReceivedCB:         stateProposalReceivedCB,
		decidedStateQueryInputsReadyCB:  decidedStateQueryInputsReadyCB,
		decidedStateReceivedCB:          decidedStateReceivedCB,
		saveProducedBlockInputsReadyCB:  saveProducedBlockInputsReadyCB,
		saveProducedBlockDoneCB:         saveProducedBlockDoneCB,
	}
}

func (sub *syncSMImpl) ProposedBaseAnchorOutputReceived(baseAnchorOutput *isc.AnchorOutputWithID) gpa.OutMessages {
	if sub.proposedBaseAnchorOutput != nil {
		return nil
	}
	sub.proposedBaseAnchorOutput = baseAnchorOutput
	return sub.stateProposalQueryInputsReadyCB(sub.proposedBaseAnchorOutput)
}

func (sub *syncSMImpl) StateProposalConfirmedByStateMgr() gpa.OutMessages {
	if sub.stateProposalReceived {
		return nil
	}
	sub.stateProposalReceived = true
	return sub.stateProposalReceivedCB(sub.proposedBaseAnchorOutput)
}

func (sub *syncSMImpl) DecidedVirtualStateNeeded(decidedBaseAnchorOutput *isc.AnchorOutputWithID) gpa.OutMessages {
	if sub.decidedBaseAnchorOutput != nil {
		return nil
	}
	sub.decidedBaseAnchorOutput = decidedBaseAnchorOutput
	return sub.decidedStateQueryInputsReadyCB(decidedBaseAnchorOutput)
}

func (sub *syncSMImpl) DecidedVirtualStateReceived(
	chainState state.State,
) gpa.OutMessages {
	if sub.decidedStateReceived {
		return nil
	}
	sub.decidedStateReceived = true
	return sub.decidedStateReceivedCB(chainState)
}

func (sub *syncSMImpl) BlockProduced(block state.StateDraft) gpa.OutMessages {
	if sub.producedBlockReceived {
		return nil
	}
	sub.producedBlock = block
	sub.producedBlockReceived = true
	return sub.saveProducedBlockInputsReadyCB(sub.producedBlock)
}

func (sub *syncSMImpl) BlockSaved(block state.Block) gpa.OutMessages {
	if sub.saveProducedBlockDone {
		return nil
	}
	sub.saveProducedBlockDone = true
	return sub.saveProducedBlockDoneCB(block)
}

// Try to provide useful human-readable compact status.
func (sub *syncSMImpl) String() string {
	str := "SM"
	if sub.stateProposalReceived && sub.decidedStateReceived {
		return str + statusStrOK
	}
	if sub.stateProposalReceived {
		str += "/proposal=OK"
	} else if sub.proposedBaseAnchorOutput == nil {
		str += "/proposal=WAIT[params: baseAnchorOutput]"
	} else {
		str += "/proposal=WAIT[RespFromStateMgr]"
	}
	if sub.decidedStateReceived {
		str += "/state=OK"
	} else if sub.decidedBaseAnchorOutput == nil {
		str += "/state=WAIT[acs decision]"
	} else {
		str += "/state=WAIT[RespFromStateMgr]"
	}
	if sub.saveProducedBlockDone {
		str += "/state=OK"
	} else if sub.producedBlock == nil {
		str += "/state=WAIT[BlockFromVM]"
	} else {
		str += "/state=WAIT[RespFromStateMgr]"
	}
	return str
}
