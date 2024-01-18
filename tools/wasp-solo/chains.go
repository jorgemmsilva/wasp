package main

import (
	"errors"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/solo"
)

func chainsProvider(env *solo.Solo) chaintypes.ChainsProvider {
	return func() chaintypes.Chains {
		return &soloChainsProvider{env}
	}
}

type soloChainsProvider struct{ env *solo.Solo }

// Activate implements chaintypes.Chains.
func (*soloChainsProvider) Activate(isc.ChainID) error {
	panic("unimplemented")
}

// Deactivate implements chaintypes.Chains.
func (*soloChainsProvider) Deactivate(isc.ChainID) error {
	panic("unimplemented")
}

// Get implements chaintypes.Chains.
func (s *soloChainsProvider) Get(chainID isc.ChainID) (chaintypes.Chain, error) {
	chains := s.env.GetChains()
	if chains[chainID] == nil {
		return nil, errors.New("chain not found")
	}
	return chains[chainID], nil
}

// IsArchiveNode implements chaintypes.Chains.
func (*soloChainsProvider) IsArchiveNode() bool {
	return false
}

// ValidatorAddress implements chaintypes.Chains.
func (*soloChainsProvider) ValidatorAddress() iotago.Address {
	panic("unimplemented")
}
