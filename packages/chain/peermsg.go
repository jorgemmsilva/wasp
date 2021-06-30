// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package chain

import (
	"io"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm"
	"go.dedis.ch/kyber/v3/sign/tbls"
)

// Message types for the committee communications.
const (
	MsgGetBlock = 1 + peering.FirstUserMsgCode + iota
	MsgBlock
	MsgSignedResult
	MsgOffLedgerRequest
	MsgMissingRequestIDs
	MsgMissingRequest
)

type TimerTick int

type SignedResultMsg struct {
	SenderIndex  uint16
	ChainInputID ledgerstate.OutputID
	EssenceHash  hashing.HashValue
	SigShare     tbls.SigShare
}

// GetBlockMsg StateManager queries specific block data from another peer (access node)
type GetBlockMsg struct {
	SenderNetID string
	BlockIndex  uint32
}

// BlockMsg StateManager in response to GetBlockMsg sends block data to the querying node's StateManager
type BlockMsg struct {
	SenderNetID string
	BlockBytes  []byte
}

// DismissChainMsg sent by component to the chain core in case of major setback
type DismissChainMsg struct {
	Reason string
}

// StateTransitionMsg Notifies chain about changed state
type StateTransitionMsg struct {
	// new variable state
	State state.VirtualState
	// corresponding state transaction
	StateOutput *ledgerstate.AliasOutput
	//
	StateTimestamp time.Time
}

// StateCandidateMsg Consensus sends the finalized next state to StateManager
type StateCandidateMsg struct {
	State             state.VirtualState
	ApprovingOutputID ledgerstate.OutputID
}

// VMResultMsg Consensus -> Consensus. VM sends result of async task started by Consensus to itself
type VMResultMsg struct {
	Task *vm.VMTask
}

// AsynchronousCommonSubsetMsg
type AsynchronousCommonSubsetMsg struct {
	ProposedBatchesBin [][]byte
	SessionID          uint64
}

// InclusionStateMsg txstream plugin sends inclusions state of the transaction to ConsensusOld
type InclusionStateMsg struct {
	TxID  ledgerstate.TransactionID
	State ledgerstate.InclusionState
}

// StateMsg txstream plugin sends the only existing AliasOutput in the chain's address to StateManager
type StateMsg struct {
	ChainOutput *ledgerstate.AliasOutput
	Timestamp   time.Time
}

func (msg *GetBlockMsg) Write(w io.Writer) error {
	return util.WriteUint32(w, msg.BlockIndex)
}

func (msg *GetBlockMsg) Read(r io.Reader) error {
	return util.ReadUint32(r, &msg.BlockIndex)
}

func (msg *BlockMsg) Write(w io.Writer) error {
	return util.WriteBytes32(w, msg.BlockBytes)
}

func (msg *BlockMsg) Read(r io.Reader) error {
	var err error
	if msg.BlockBytes, err = util.ReadBytes32(r); err != nil {
		return err
	}
	return nil
}

func (msg *SignedResultMsg) Write(w io.Writer) error {
	if _, err := w.Write(msg.EssenceHash[:]); err != nil {
		return err
	}
	if err := util.WriteBytes16(w, msg.SigShare); err != nil {
		return err
	}
	if _, err := w.Write(msg.ChainInputID[:]); err != nil {
		return err
	}
	return nil
}

func (msg *SignedResultMsg) Read(r io.Reader) error {
	if err := util.ReadHashValue(r, &msg.EssenceHash); err != nil {
		return err
	}
	var err error
	if msg.SigShare, err = util.ReadBytes16(r); err != nil {
		return err
	}
	if err := util.ReadOutputID(r, &msg.ChainInputID); /* nolint:revive */ err != nil {
		return err
	}
	return nil
}
