// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package cmtLog_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/chain/cmtLog"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/testutil/testiotago"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
	"github.com/iotaledger/wasp/packages/testutil/testpeers"
)

// TODO: Test should involve suspend/resume.

func TestBasic(t *testing.T) {
	type test struct {
		n int
		f int
	}
	tests := []test{
		{n: 4, f: 1},
	}
	for _, tst := range tests {
		t.Run(
			fmt.Sprintf("N=%v,F=%v", tst.n, tst.f),
			func(tt *testing.T) { testBasic(tt, tst.n, tst.f) })
	}
}

func testBasic(t *testing.T, n, f int) {
	log := testlogger.NewLogger(t)
	defer log.Sync()
	//
	// Chain identifiers.
	aliasID := testiotago.RandAliasID()
	chainID := isc.ChainIDFromAliasID(aliasID)
	governor := cryptolib.NewKeyPair()
	//
	// Node identities.
	_, peerIdentities := testpeers.SetupKeys(uint16(n))
	peerPubKeys := testpeers.PublicKeys(peerIdentities)
	//
	// Committee.
	committeeAddress, committeeKeyShares := testpeers.SetupDkgTrivial(t, n, f, peerIdentities, nil)
	//
	// Construct the algorithm nodes.
	gpaNodeIDs := gpa.NodeIDsFromPublicKeys(peerPubKeys)
	gpaNodes := map[gpa.NodeID]gpa.GPA{}
	for i := range gpaNodeIDs {
		dkShare, err := committeeKeyShares[i].LoadDKShare(committeeAddress)
		require.NoError(t, err)
		consensusStateRegistry := testutil.NewConsensusStateRegistry() // Empty store in this case.
		cmtLogInst, err := cmtLog.New(gpaNodeIDs[i], chainID, dkShare, consensusStateRegistry, gpa.NodeIDFromPublicKey, log.Named(fmt.Sprintf("N%v", i)))
		require.NoError(t, err)
		gpaNodes[gpaNodeIDs[i]] = cmtLogInst.AsGPA()
	}
	gpaTC := gpa.NewTestContext(gpaNodes)
	//
	// Start the algorithms.
	gpaTC.RunAll()
	gpaTC.PrintAllStatusStrings("Initial", t.Logf)
	//
	// Provide first alias output. Consensus should be sent now.
	ao1 := randomAliasOutputWithID(aliasID, governor.Address(), committeeAddress, 1)
	gpaTC.WithInputs(inputAliasOutputConfirmed(gpaNodes, ao1)).RunAll()
	gpaTC.PrintAllStatusStrings("After AO1Recv", t.Logf)
	cons1 := gpaNodes[gpaNodeIDs[0]].Output().(*cmtLog.Output)
	for _, n := range gpaNodes {
		require.NotNil(t, n.Output())
		require.Equal(t, cons1, n.Output())
	}
	//
	// Consensus results received (consumed ao1, produced ao2).
	ao2 := randomAliasOutputWithID(aliasID, governor.Address(), committeeAddress, 2)
	gpaTC.WithInputs(inputConsensusOutput(gpaNodes, cons1, ao2)).RunAll()
	gpaTC.PrintAllStatusStrings("After gpaMsgsAO2Cons", t.Logf)
	cons2 := gpaNodes[gpaNodeIDs[0]].Output().(*cmtLog.Output)
	require.Equal(t, cons2.GetLogIndex(), cons1.GetLogIndex().Next())
	require.Equal(t, cons2.GetBaseAliasOutput(), ao2)
	for _, n := range gpaNodes {
		require.NotNil(t, n.Output())
		require.Equal(t, cons2, n.Output())
	}
	//
	// AO Confirmed received (nothing changes, we are ahead of it)
	gpaTC.WithInputs(inputAliasOutputConfirmed(gpaNodes, ao2)).RunAll()
	gpaTC.PrintAllStatusStrings("After gpaMsgsAO2Recv", t.Logf)
	for _, n := range gpaNodes {
		require.NotNil(t, n.Output())
		require.Equal(t, cons2, n.Output())
	}
	//
	// pass another confirmed // TODO: WTF??
}

////////////////////////////////////////////////////////////////////////////////
// Helper functions.

func inputAliasOutputConfirmed(gpaNodes map[gpa.NodeID]gpa.GPA, ao *isc.AliasOutputWithID) map[gpa.NodeID]gpa.Input {
	inputs := map[gpa.NodeID]gpa.Input{}
	for n := range gpaNodes {
		inputs[n] = cmtLog.NewInputAliasOutputConfirmed(ao)
	}
	return inputs
}

func inputConsensusOutput(gpaNodes map[gpa.NodeID]gpa.GPA, consReq *cmtLog.Output, nextAO *isc.AliasOutputWithID) map[gpa.NodeID]gpa.Input {
	inputs := map[gpa.NodeID]gpa.Input{}
	for n := range gpaNodes {
		inputs[n] = cmtLog.NewInputConsensusOutputDone(consReq.GetLogIndex(), consReq.GetBaseAliasOutput().OutputID(), consReq.GetBaseAliasOutput().OutputID(), nextAO)
	}
	return inputs
}

func randomAliasOutputWithID(aliasID iotago.AliasID, governorAddress, stateAddress iotago.Address, stateIndex uint32) *isc.AliasOutputWithID {
	outputID := testiotago.RandOutputID()
	aliasOutput := &iotago.AliasOutput{
		AliasID:    aliasID,
		StateIndex: stateIndex,
		Conditions: iotago.UnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{Address: stateAddress},
			&iotago.GovernorAddressUnlockCondition{Address: governorAddress},
		},
	}
	return isc.NewAliasOutputWithID(aliasOutput, outputID)
}
