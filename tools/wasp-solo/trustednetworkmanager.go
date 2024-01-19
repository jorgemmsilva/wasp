package main

import (
	"context"
	"errors"

	"github.com/samber/lo"

	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/solo"
)

func trustedNetworkManager(env *solo.Solo) peering.TrustedNetworkManager {
	return &soloTrustedNetworkManager{env}
}

type soloTrustedNetworkManager struct{ env *solo.Solo }

// DistrustPeer implements peering.TrustedNetworkManager.
func (*soloTrustedNetworkManager) DistrustPeer(pubKey *cryptolib.PublicKey) (*peering.TrustedPeer, error) {
	panic("unimplemented")
}

// IsTrustedPeer implements peering.TrustedNetworkManager.
func (*soloTrustedNetworkManager) IsTrustedPeer(pubKey *cryptolib.PublicKey) error {
	if !pubKey.Equals(peer.GetPublicKey()) {
		return errors.New("not trusted")
	}
	return nil
}

// TrustPeer implements peering.TrustedNetworkManager.
func (*soloTrustedNetworkManager) TrustPeer(name string, pubKey *cryptolib.PublicKey, peeringURL string) (*peering.TrustedPeer, error) {
	panic("unimplemented")
}

// TrustedPeers implements peering.TrustedNetworkManager.
func (*soloTrustedNetworkManager) TrustedPeers() ([]*peering.TrustedPeer, error) {
	return []*peering.TrustedPeer{peering.NewTrustedPeer("solo", peer.GetPublicKey(), "")}, nil
}

// TrustedPeersByPubKeyOrName implements peering.TrustedNetworkManager.
func (s *soloTrustedNetworkManager) TrustedPeersByPubKeyOrName(pubKeysOrNames []string) ([]*peering.TrustedPeer, error) {
	return peering.QueryByPubKeyOrName(lo.Must(s.TrustedPeers()), pubKeysOrNames)
}

// TrustedPeersListener implements peering.TrustedNetworkManager.
func (*soloTrustedNetworkManager) TrustedPeersListener(callback func([]*peering.TrustedPeer)) context.CancelFunc {
	panic("unimplemented")
}
