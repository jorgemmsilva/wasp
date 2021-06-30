// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package testpeers

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/wasp/packages/coretypes"
	"github.com/iotaledger/wasp/packages/dkg"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/tcrypto"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
	"github.com/stretchr/testify/require"
)

func SetupKeys(peerCount uint16) ([]string, []*ed25519.KeyPair) {
	peerNetIDs := make([]string, peerCount)
	peerIdentities := make([]*ed25519.KeyPair, peerCount)
	for i := range peerNetIDs {
		peerIdentity := ed25519.GenerateKeyPair()
		peerNetIDs[i] = fmt.Sprintf("P%02d", i)
		peerIdentities[i] = &peerIdentity
	}
	return peerNetIDs, peerIdentities
}

func PublicKeys(peerIdentities []*ed25519.KeyPair) []ed25519.PublicKey {
	pubKeys := make([]ed25519.PublicKey, len(peerIdentities))
	for i := range pubKeys {
		pubKeys[i] = peerIdentities[i].PublicKey
	}
	return pubKeys
}

func SetupDkg(
	t *testing.T,
	threshold uint16,
	peerNetIDs []string,
	peerIdentities []*ed25519.KeyPair,
	suite tcrypto.Suite,
	log *logger.Logger,
) (ledgerstate.Address, []coretypes.DKShareRegistryProvider) {
	timeout := 100 * time.Second
	networkProviders := SetupNet(peerNetIDs, peerIdentities, testutil.NewPeeringNetReliable(), log)
	//
	// Initialize the DKG subsystem in each node.
	dkgNodes := make([]*dkg.Node, len(peerNetIDs))
	registries := make([]coretypes.DKShareRegistryProvider, len(peerNetIDs))
	for i := range peerNetIDs {
		registries[i] = testutil.NewDkgRegistryProvider(suite)
		dkgNode, err := dkg.NewNode(
			peerIdentities[i], networkProviders[i], registries[i],
			testlogger.WithLevel(log.With("NetID", peerNetIDs[i]), logger.LevelError, false),
		)
		require.NoError(t, err)
		dkgNodes[i] = dkgNode
	}
	//
	// Initiate the key generation from some client node.
	dkShare, err := dkgNodes[0].GenerateDistributedKey(
		peerNetIDs,
		PublicKeys(peerIdentities),
		threshold,
		100*time.Second,
		200*time.Second,
		timeout,
	)
	require.Nil(t, err)
	require.NotNil(t, dkShare.Address)
	require.NotNil(t, dkShare.SharedPublic)
	return dkShare.Address, registries
}

func SetupDkgPregenerated(
	t *testing.T,
	threshold uint16,
	peerNetIDs []string,
	suite tcrypto.Suite,
) (ledgerstate.Address, []coretypes.DKShareRegistryProvider) {
	var err error
	var serializedDks [][]byte = pregeneratedDksRead(uint16(len(peerNetIDs)))
	dks := make([]*tcrypto.DKShare, len(serializedDks))
	registries := make([]coretypes.DKShareRegistryProvider, len(peerNetIDs))
	for i := range dks {
		dks[i], err = tcrypto.DKShareFromBytes(serializedDks[i], suite)
		if i > 0 {
			// It was removed to decrease the serialized size.
			dks[i].PublicCommits = dks[0].PublicCommits
			dks[i].PublicShares = dks[0].PublicShares
		}
		require.Nil(t, err)
		registries[i] = testutil.NewDkgRegistryProvider(suite)
		require.Nil(t, registries[i].SaveDKShare(dks[i]))
	}
	require.Equal(t, dks[0].T, threshold, "dks was pregenerated for different threshold (T=%v)", dks[0].T)
	return dks[0].Address, registries
}

func SetupNet(
	peerNetIDs []string,
	peerIdentities []*ed25519.KeyPair,
	behavior testutil.PeeringNetBehavior,
	log *logger.Logger,
) []peering.NetworkProvider {
	peeringNetwork := testutil.NewPeeringNetwork(
		peerNetIDs, peerIdentities, 10000, behavior,
		testlogger.WithLevel(log, logger.LevelWarn, false),
	)
	return peeringNetwork.NetworkProviders()
}
