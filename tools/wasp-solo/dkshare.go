package main

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/testutil/testpeers"
)

var (
	peer                    = cryptolib.KeyPairFromSeed(cryptolib.SeedFromBytes([]byte("asdf")))
	dkShareAddress          iotago.Address
	dkShareRegistryProvider registry.DKShareRegistryProvider
)

func initDKShare(env *solo.Solo) {
	addr, dkShares := testpeers.SetupDkgTrivial(env.T, 1, 0, []*cryptolib.KeyPair{peer}, nil)
	dkShareRegistryProvider = dkShares[0]
	dkShareAddress = addr
}

func getCommitteeInfo() *chaintypes.CommitteeInfo {
	return &chaintypes.CommitteeInfo{
		Address:       dkShareAddress,
		Size:          1,
		Quorum:        1,
		QuorumIsAlive: true,
		PeerStatus: []*chaintypes.PeerStatus{{
			Name:       "solo",
			Index:      0,
			PubKey:     peer.GetPublicKey(),
			PeeringURL: "",
			Connected:  true,
		}},
	}
}

func getChainNodes() []peering.PeerStatusProvider {
	return []peering.PeerStatusProvider{&peerStatusProvider{}}
}
