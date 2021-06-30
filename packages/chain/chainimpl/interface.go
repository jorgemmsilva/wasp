// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package chainimpl

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/wasp/packages/chain"
	"github.com/iotaledger/wasp/packages/coretypes"
	"github.com/iotaledger/wasp/packages/coretypes/chainid"
	"github.com/iotaledger/wasp/packages/coretypes/coreutil"
	"github.com/iotaledger/wasp/packages/coretypes/request"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/publisher"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/processors"
)

func (c *chainObj) ID() *chainid.ChainID {
	return &c.chainID
}

func (c *chainObj) GlobalStateSync() coreutil.ChainStateSync {
	return c.chainStateSync
}

func (c *chainObj) GetCommitteeInfo() *chain.CommitteeInfo {
	cmt := c.getCommittee()
	if cmt == nil {
		return nil
	}
	return &chain.CommitteeInfo{
		Address:       cmt.DKShare().Address,
		Size:          cmt.Size(),
		Quorum:        cmt.Quorum(),
		QuorumIsAlive: cmt.QuorumIsAlive(),
		PeerStatus:    cmt.PeerStatus(),
	}
}

func (c *chainObj) startTimer() {
	go func() {
		c.stateMgr.Ready().MustWait()
		tick := 0
		for !c.IsDismissed() {
			time.Sleep(chain.TimerTickPeriod)
			c.ReceiveMessage(chain.TimerTick(tick))
			tick++
		}
	}()
}

func (c *chainObj) Dismiss(reason string) {
	c.log.Infof("Dismiss chain. Reason: '%s'", reason)

	c.dismissOnce.Do(func() {
		c.dismissed.Store(true)

		close(c.chMsg)

		c.mempool.Close()
		c.stateMgr.Close()
		cmt := c.getCommittee()
		if cmt != nil {
			cmt.Close()
		}
		if c.consensus != nil {
			c.consensus.Close()
		}
		c.eventRequestProcessed.DetachAll()
		c.eventChainTransition.DetachAll()
		c.eventSynced.DetachAll()
	})

	publisher.Publish("dismissed_chain", c.chainID.Base58())
}

func (c *chainObj) IsDismissed() bool {
	return c.dismissed.Load()
}

func (c *chainObj) ReceiveMessage(msg interface{}) {
	if !c.IsDismissed() {
		select {
		case c.chMsg <- msg:
		default:
			c.log.Warnf("ReceiveMessage with type '%T' failed. Retrying after %s", msg, chain.ReceiveMsgChannelRetryDelay)
			go func() {
				time.Sleep(chain.ReceiveMsgChannelRetryDelay)
				c.ReceiveMessage(msg)
			}()
		}
	}
}

// TODO make configurable
const gossipInterval = 1 * time.Second

func (c *chainObj) gossipOffLedgerRequest(req *request.RequestOffLedger) {
	msgData := chain.NewOffledgerRequestMsg(&c.chainID, req).Bytes()
	committee := c.getCommittee()
	var sendMessage func()

	if committee != nil {
		sendMessage = func() {
			committee.SendMsgToPeers(chain.MsgOffLedgerRequest, msgData, time.Now().UnixNano())
		}
	} else {
		sendMessage = func() {
			gossipUpToNPeers := parameters.GetInt(parameters.OffledgerGossipUpToNPeers)
			(*c.peers).SendMsgToRandomPeersSimple(uint16(gossipUpToNPeers), chain.MsgOffLedgerRequest, msgData)
		}
	}
	ticker := time.NewTicker(gossipInterval)
	// TODO expire after a set amount of time
	go func() {
		for {
			select {
			case <-ticker.C:
				// check if processed (request already left the mempool)
				if !c.mempool.HasRequest(req.ID()) {
					ticker.Stop()
					return
				}
				sendMessage()
			}
		}
	}()
}

func (c *chainObj) ReceiveOffLedgerRequest(req *request.RequestOffLedger) {
	if !c.mempool.ReceiveRequest(req) {
		return
	}
	c.gossipOffLedgerRequest(req)
}

// SendMissingRequestsToPeer sends the requested missing requests by a peer
func (c *chainObj) SendMissingRequestsToPeer(msg chain.MissingRequestIDsMsg, peerID string) {
	for _, reqID := range msg.IDs {
		req := c.mempool.GetRequest(reqID)
		msg := chain.NewMissingRequestMsg(req)
		(*c.peers).SendSimple(peerID, chain.MsgMissingRequest, msg.Bytes())
	}
}

func (c *chainObj) ReceiveTransaction(tx *ledgerstate.Transaction) {
	c.log.Debugf("ReceiveTransaction: %s", tx.ID().Base58())
	reqs, err := request.RequestsOnLedgerFromTransaction(tx, c.chainID.AsAddress())
	if err != nil {
		c.log.Warnf("failed to parse transaction %s: %v", tx.ID().Base58(), err)
		return
	}
	for _, req := range reqs {
		c.ReceiveRequest(req)
	}
	if chainOut := transaction.GetAliasOutput(tx, c.chainID.AsAddress()); chainOut != nil {
		c.ReceiveState(chainOut, tx.Essence().Timestamp())
	}
}

func (c *chainObj) ReceiveRequest(req coretypes.Request) {
	c.log.Debugf("ReceiveRequest: %s", req.ID())
	c.mempool.ReceiveRequests(req)
}

func (c *chainObj) ReceiveState(stateOutput *ledgerstate.AliasOutput, timestamp time.Time) {
	c.log.Debugf("ReceiveState #%d: outputID: %s, stateAddr: %s",
		stateOutput.GetStateIndex(), coretypes.OID(stateOutput.ID()), stateOutput.GetStateAddress().Base58())
	c.ReceiveMessage(&chain.StateMsg{
		ChainOutput: stateOutput,
		Timestamp:   timestamp,
	})
}

func (c *chainObj) ReceiveInclusionState(txID ledgerstate.TransactionID, inclusionState ledgerstate.InclusionState) {
	c.ReceiveMessage(&chain.InclusionStateMsg{
		TxID:  txID,
		State: inclusionState,
	}) // TODO special entry point
}

func (c *chainObj) ReceiveOutput(output ledgerstate.Output) {
	c.stateMgr.EventOutputMsg(output)
}

func (c *chainObj) BlobCache() coretypes.BlobCache {
	return c.blobProvider
}

func (c *chainObj) GetRequestProcessingStatus(reqID coretypes.RequestID) chain.RequestProcessingStatus {
	if c.IsDismissed() {
		return chain.RequestProcessingStatusUnknown
	}
	if c.consensus != nil {
		if c.mempool.HasRequest(reqID) {
			return chain.RequestProcessingStatusBacklog
		}
	}
	c.stateReader.SetBaseline()
	processed, err := blocklog.IsRequestProcessed(c.stateReader.KVStoreReader(), &reqID)
	if err != nil || !processed {
		return chain.RequestProcessingStatusUnknown
	}
	return chain.RequestProcessingStatusCompleted
}

func (c *chainObj) Processors() *processors.Cache {
	return c.procset
}

func (c *chainObj) EventRequestProcessed() *events.Event {
	return c.eventRequestProcessed
}

func (c *chainObj) RequestProcessed() *events.Event {
	return c.eventRequestProcessed
}

func (c *chainObj) ChainTransition() *events.Event {
	return c.eventChainTransition
}

func (c *chainObj) StateSynced() *events.Event {
	return c.eventSynced
}

func (c *chainObj) Events() chain.ChainEvents {
	return c
}

// GetStateReader returns a new copy of the optimistic state reader, with own baseline
func (c *chainObj) GetStateReader() state.OptimisticStateReader {
	return state.NewOptimisticStateReader(c.db, c.chainStateSync)
}

func (c *chainObj) Log() *logger.Logger {
	return c.log
}
