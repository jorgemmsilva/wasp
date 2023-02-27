// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package chainMgr_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/chain/chainMgr"
	"github.com/iotaledger/wasp/packages/chain/cons"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/origin"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/testutil/testchain"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
	"github.com/iotaledger/wasp/packages/testutil/testpeers"
	"github.com/iotaledger/wasp/packages/utxodb"
)

func TestBasic(t *testing.T) {
	type test struct {
		n int
		f int
	}
	tests := []test{
		{n: 1, f: 0},   // Low N.
		{n: 2, f: 0},   // Low N.
		{n: 3, f: 0},   // Low N.
		{n: 4, f: 1},   // Smallest robust cluster.
		{n: 10, f: 3},  // Typical config.
		{n: 31, f: 10}, // Large cluster.
	}
	for i := range tests {
		tst := tests[i]
		t.Run(
			fmt.Sprintf("N=%v,F=%v", tst.n, tst.f),
			func(tt *testing.T) { testBasic(tt, tst.n, tst.f) },
		)
	}
}

func testBasic(t *testing.T, n, f int) {
	log := testlogger.NewLogger(t)
	defer log.Sync()
	//
	// Create ledger accounts.
	utxoDB := utxodb.New(utxodb.DefaultInitParams())
	originator := cryptolib.NewKeyPair()
	_, err := utxoDB.GetFundsFromFaucet(originator.Address())
	require.NoError(t, err)
	//
	// Node identities and DKG.
	_, peerIdentities := testpeers.SetupKeys(uint16(n))
	nodeIDs := make([]gpa.NodeID, len(peerIdentities))
	for i, pid := range peerIdentities {
		nodeIDs[i] = gpa.NodeIDFromPublicKey(pid.GetPublicKey())
	}
	cmtAddrA, dkRegs := testpeers.SetupDkgTrivial(t, n, f, peerIdentities, nil)
	cmtAddrB, dkRegs := testpeers.SetupDkgTrivial(t, n, f, peerIdentities, dkRegs)
	require.NotNil(t, cmtAddrA)
	require.NotNil(t, cmtAddrB)
	//
	// Chain identifiers.
	tcl := testchain.NewTestChainLedger(t, utxoDB, originator)
	_, originAO, chainID := tcl.MakeTxChainOrigin(cmtAddrA)
	//
	// Construct the nodes.
	nodes := map[gpa.NodeID]gpa.GPA{}
	for i, nid := range nodeIDs {
		consensusStateRegistry := testutil.NewConsensusStateRegistry()
		cm, err := chainMgr.New(nid, chainID, consensusStateRegistry, dkRegs[i], gpa.NodeIDFromPublicKey, func(pk []*cryptolib.PublicKey) {}, log.Named(nid.ShortString()))
		require.NoError(t, err)
		nodes[nid] = cm.AsGPA()
	}
	tc := gpa.NewTestContext(nodes)
	tc.PrintAllStatusStrings("Started", t.Logf)
	//
	// Provide initial AO.
	initAOInputs := map[gpa.NodeID]gpa.Input{}
	for nid := range nodes {
		initAOInputs[nid] = chainMgr.NewInputAliasOutputConfirmed(originAO)
	}
	tc.WithInputs(initAOInputs)
	tc.RunAll()
	tc.PrintAllStatusStrings("Initial AO received", t.Logf)
	for _, n := range nodes {
		out := n.Output().(*chainMgr.Output)
		require.Len(t, out.NeedPublishTX(), 0)
		require.NotNil(t, out.NeedConsensus())
		require.Equal(t, originAO, out.NeedConsensus().BaseAliasOutput)
		require.Equal(t, uint32(1), out.NeedConsensus().LogIndex.AsUint32())
		require.Equal(t, cmtAddrA, &out.NeedConsensus().CommitteeAddr)
	}
	//
	// Provide consensus output.
	step2AO, step2TX := tcl.FakeTX(originAO, cmtAddrA)
	for nid := range nodes {
		consReq := nodes[nid].Output().(*chainMgr.Output).NeedConsensus()
		fake2ST := origin.InitChain(state.NewStore(mapdb.NewMapDB()), nil, 0).NewOriginStateDraft()
		tc.WithInput(nid, chainMgr.NewInputConsensusOutputDone( // TODO: Consider the SKIP cases as well.
			*cmtAddrA.(*iotago.Ed25519Address),
			consReq.LogIndex, consReq.BaseAliasOutput.OutputID(),
			&cons.Result{
				Transaction:     step2TX,
				StateDraft:      fake2ST,
				BaseAliasOutput: consReq.BaseAliasOutput.OutputID(),
				NextAliasOutput: step2AO,
			},
		))
	}
	tc.RunAll()
	tc.PrintAllStatusStrings("Consensus done", t.Logf)
	for nodeID, n := range nodes {
		out := n.Output().(*chainMgr.Output)
		t.Logf("node=%v should have 1 TX to publish, have out=%v", nodeID, out)
		require.Len(t, out.NeedPublishTX(), 1, "node=%v should have 1 TX to publish, have out=%v", nodeID, out)
		require.Equal(t, step2TX, out.NeedPublishTX()[step2AO.TransactionID()].Tx)
		require.Equal(t, originAO.OutputID(), out.NeedPublishTX()[step2AO.TransactionID()].BaseAliasOutputID)
		require.Equal(t, cmtAddrA, &out.NeedPublishTX()[step2AO.TransactionID()].CommitteeAddr)
		require.NotNil(t, out.NeedConsensus())
		require.Equal(t, step2AO, out.NeedConsensus().BaseAliasOutput)
		require.Equal(t, uint32(2), out.NeedConsensus().LogIndex.AsUint32())
		require.Equal(t, cmtAddrA, &out.NeedConsensus().CommitteeAddr)
	}
	//
	// Say TX is published
	for nid := range nodes {
		consReq := nodes[nid].Output().(*chainMgr.Output).NeedPublishTX()[step2AO.TransactionID()]
		tc.WithInput(nid, chainMgr.NewInputChainTxPublishResult(consReq.CommitteeAddr, consReq.TxID, consReq.NextAliasOutput, true))
	}
	tc.RunAll()
	tc.PrintAllStatusStrings("TX Published", t.Logf)
	for _, n := range nodes {
		out := n.Output().(*chainMgr.Output)
		require.Len(t, out.NeedPublishTX(), 0)
		require.NotNil(t, out.NeedConsensus())
		require.Equal(t, step2AO, out.NeedConsensus().BaseAliasOutput)
		require.Equal(t, uint32(2), out.NeedConsensus().LogIndex.AsUint32())
		require.Equal(t, cmtAddrA, &out.NeedConsensus().CommitteeAddr)
	}
	//
	// Say TX is confirmed.
	for nid := range nodes {
		tc.WithInput(nid, chainMgr.NewInputAliasOutputConfirmed(step2AO))
	}
	tc.RunAll()
	tc.PrintAllStatusStrings("TX Published and Confirmed", t.Logf)
	for _, n := range nodes {
		out := n.Output().(*chainMgr.Output)
		require.Len(t, out.NeedPublishTX(), 0)
		require.NotNil(t, out.NeedConsensus())
		require.Equal(t, step2AO, out.NeedConsensus().BaseAliasOutput)
		require.Equal(t, uint32(2), out.NeedConsensus().LogIndex.AsUint32())
		require.Equal(t, cmtAddrA, &out.NeedConsensus().CommitteeAddr)
	}
	//
	// Make external committee rotation.
	rotateAO, _ := tcl.FakeTX(step2AO, cmtAddrB)
	for nid := range nodes {
		tc.WithInput(nid, chainMgr.NewInputAliasOutputConfirmed(rotateAO))
	}
	tc.RunAll()
	tc.PrintAllStatusStrings("After external rotation", t.Logf)
	for _, n := range nodes {
		out := n.Output().(*chainMgr.Output)
		require.Len(t, out.NeedPublishTX(), 0)
		require.NotNil(t, out.NeedConsensus())
		require.Equal(t, rotateAO, out.NeedConsensus().BaseAliasOutput)
		require.Equal(t, uint32(1), out.NeedConsensus().LogIndex.AsUint32())
		require.Equal(t, cmtAddrB, &out.NeedConsensus().CommitteeAddr)
	}
}
