package main

import (
	"context"
	"time"

	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/solo"
)

func networkProvider(env *solo.Solo) peering.NetworkProvider {
	return &soloNetworkProvider{env}
}

type soloNetworkProvider struct{ env *solo.Solo }

// Await implements peering.PeerSender.
func (*soloNetworkProvider) Await(timeout time.Duration) error {
	return nil
}

// Close implements peering.PeerSender.
func (*soloNetworkProvider) Close() {
}

// IsAlive implements peering.PeerSender.
func (*soloNetworkProvider) IsAlive() bool {
	return true
}

// PeeringURL implements peering.PeerSender.
func (*soloNetworkProvider) PeeringURL() string {
	return ""
}

// PubKey implements peering.PeerSender.
func (*soloNetworkProvider) PubKey() *cryptolib.PublicKey {
	return peer.GetPublicKey()
}

// SendMsg implements peering.PeerSender.
func (*soloNetworkProvider) SendMsg(msg *peering.PeerMessageData) {
	panic("unimplemented")
}

// Status implements peering.PeerSender.
func (s *soloNetworkProvider) Status() peering.PeerStatusProvider {
	return &peerStatusProvider{}
}

// Attach implements peering.NetworkProvider.
func (*soloNetworkProvider) Attach(peeringID *peering.PeeringID, receiver byte, callback func(recv *peering.PeerMessageIn)) context.CancelFunc {
	return func() {
	}
}

// PeerByPubKey implements peering.NetworkProvider.
func (*soloNetworkProvider) PeerByPubKey(peerPub *cryptolib.PublicKey) (peering.PeerSender, error) {
	panic("unimplemented")
}

// PeerDomain implements peering.NetworkProvider.
func (*soloNetworkProvider) PeerDomain(peeringID peering.PeeringID, peerAddrs []*cryptolib.PublicKey) (peering.PeerDomainProvider, error) {
	panic("unimplemented")
}

// PeerGroup implements peering.NetworkProvider.
func (*soloNetworkProvider) PeerGroup(peeringID peering.PeeringID, peerPubKeys []*cryptolib.PublicKey) (peering.GroupProvider, error) {
	panic("unimplemented")
}

// PeerStatus implements peering.NetworkProvider.
func (s *soloNetworkProvider) PeerStatus() []peering.PeerStatusProvider {
	return []peering.PeerStatusProvider{&peerStatusProvider{}}
}

// Run implements peering.NetworkProvider.
func (*soloNetworkProvider) Run(ctx context.Context) {
	panic("unimplemented")
}

// Self implements peering.NetworkProvider.
func (s *soloNetworkProvider) Self() peering.PeerSender {
	return s
}

// SendMsgByPubKey implements peering.NetworkProvider.
func (*soloNetworkProvider) SendMsgByPubKey(pubKey *cryptolib.PublicKey, msg *peering.PeerMessageData) {
	panic("unimplemented")
}

type peerStatusProvider struct{}

// IsAlive implements peering.PeerStatusProvider.
func (*peerStatusProvider) IsAlive() bool {
	return true
}

// Name implements peering.PeerStatusProvider.
func (*peerStatusProvider) Name() string {
	return "solo"
}

// NumUsers implements peering.PeerStatusProvider.
func (*peerStatusProvider) NumUsers() int {
	return 1
}

// PeeringURL implements peering.PeerStatusProvider.
func (*peerStatusProvider) PeeringURL() string {
	return ""
}

// PubKey implements peering.PeerStatusProvider.
func (*peerStatusProvider) PubKey() *cryptolib.PublicKey {
	return peer.GetPublicKey()
}
