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
	ProposedBaseAccountOutputReceived(baseAccountOutput *isc.AccountOutputWithID) gpa.OutMessages
	StateProposalConfirmedByStateMgr() gpa.OutMessages
	//
	// Decided state.
	DecidedVirtualStateNeeded(decidedBaseAccountOutput *isc.AccountOutputWithID) gpa.OutMessages
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
	proposedBaseAccountOutput         *isc.AccountOutputWithID
	stateProposalQueryInputsReadyCB func(baseAccountOutput *isc.AccountOutputWithID) gpa.OutMessages
	stateProposalReceived           bool
	stateProposalReceivedCB         func(proposedAccountOutput *isc.AccountOutputWithID) gpa.OutMessages
	//
	// Query for a decided Virtual State.
	decidedBaseAccountOutput         *isc.AccountOutputWithID
	decidedStateQueryInputsReadyCB func(decidedBaseAccountOutput *isc.AccountOutputWithID) gpa.OutMessages
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
	stateProposalQueryInputsReadyCB func(baseAccountOutput *isc.AccountOutputWithID) gpa.OutMessages,
	stateProposalReceivedCB func(proposedAccountOutput *isc.AccountOutputWithID) gpa.OutMessages,
	decidedStateQueryInputsReadyCB func(decidedBaseAccountOutput *isc.AccountOutputWithID) gpa.OutMessages,
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

func (sub *syncSMImpl) ProposedBaseAccountOutputReceived(baseAccountOutput *isc.AccountOutputWithID) gpa.OutMessages {
	if sub.proposedBaseAccountOutput != nil {
		return nil
	}
	sub.proposedBaseAccountOutput = baseAccountOutput
	return sub.stateProposalQueryInputsReadyCB(sub.proposedBaseAccountOutput)
}

func (sub *syncSMImpl) StateProposalConfirmedByStateMgr() gpa.OutMessages {
	if sub.stateProposalReceived {
		return nil
	}
	sub.stateProposalReceived = true
	return sub.stateProposalReceivedCB(sub.proposedBaseAccountOutput)
}

func (sub *syncSMImpl) DecidedVirtualStateNeeded(decidedBaseAccountOutput *isc.AccountOutputWithID) gpa.OutMessages {
	if sub.decidedBaseAccountOutput != nil {
		return nil
	}
	sub.decidedBaseAccountOutput = decidedBaseAccountOutput
	return sub.decidedStateQueryInputsReadyCB(decidedBaseAccountOutput)
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
	} else if sub.proposedBaseAccountOutput == nil {
		str += "/proposal=WAIT[params: baseAccountOutput]"
	} else {
		str += "/proposal=WAIT[RespFromStateMgr]"
	}
	if sub.decidedStateReceived {
		str += "/state=OK"
	} else if sub.decidedBaseAccountOutput == nil {
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
