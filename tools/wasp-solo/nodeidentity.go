package main

import (
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/solo"
)

func nodeIdentityProvider(env *solo.Solo) registry.NodeIdentityProvider {
	return &soloNodeIdentityProvider{env}
}

type soloNodeIdentityProvider struct{ env *solo.Solo }

// NodeIdentity implements registry.NodeIdentityProvider.
func (*soloNodeIdentityProvider) NodeIdentity() *cryptolib.KeyPair {
	panic("unimplemented")
}

// NodePublicKey implements registry.NodeIdentityProvider.
func (*soloNodeIdentityProvider) NodePublicKey() *cryptolib.PublicKey {
	panic("unimplemented")
}
