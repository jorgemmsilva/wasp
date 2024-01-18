package main

import (
	"errors"

	"github.com/samber/lo"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/solo"
)

func chainRecordRegistryProvider(env *solo.Solo) registry.ChainRecordRegistryProvider {
	return &soloChainRecordRegistryProvider{env}
}

type soloChainRecordRegistryProvider struct{ env *solo.Solo }

// ActivateChainRecord implements registry.ChainRecordRegistryProvider.
func (*soloChainRecordRegistryProvider) ActivateChainRecord(chainID isc.ChainID) (*registry.ChainRecord, error) {
	panic("unimplemented")
}

// AddChainRecord implements registry.ChainRecordRegistryProvider.
func (*soloChainRecordRegistryProvider) AddChainRecord(chainRecord *registry.ChainRecord) error {
	panic("unimplemented")
}

// ChainRecord implements registry.ChainRecordRegistryProvider.
func (s *soloChainRecordRegistryProvider) ChainRecord(chainID isc.ChainID) (*registry.ChainRecord, error) {
	chains := s.env.GetChains()
	if chains[chainID] == nil {
		return nil, errors.New("chain not found")
	}
	return registry.NewChainRecord(chainID, true, nil), nil
}

// ChainRecords implements registry.ChainRecordRegistryProvider.
func (s *soloChainRecordRegistryProvider) ChainRecords() (ret []*registry.ChainRecord, err error) {
	return lo.Map(lo.Values(s.env.GetChains()), func(ch *solo.Chain, _ int) *registry.ChainRecord {
		return registry.NewChainRecord(ch.ChainID, true, nil)
	}), nil
}

// DeactivateChainRecord implements registry.ChainRecordRegistryProvider.
func (*soloChainRecordRegistryProvider) DeactivateChainRecord(chainID isc.ChainID) (*registry.ChainRecord, error) {
	panic("unimplemented")
}

// DeleteChainRecord implements registry.ChainRecordRegistryProvider.
func (*soloChainRecordRegistryProvider) DeleteChainRecord(chainID isc.ChainID) error {
	panic("unimplemented")
}

// Events implements registry.ChainRecordRegistryProvider.
func (*soloChainRecordRegistryProvider) Events() *registry.ChainRecordRegistryEvents {
	panic("unimplemented")
}

// ForEachActiveChainRecord implements registry.ChainRecordRegistryProvider.
func (*soloChainRecordRegistryProvider) ForEachActiveChainRecord(consumer func(*registry.ChainRecord) bool) error {
	panic("unimplemented")
}

// UpdateChainRecord implements registry.ChainRecordRegistryProvider.
func (*soloChainRecordRegistryProvider) UpdateChainRecord(chainID isc.ChainID, f func(*registry.ChainRecord) bool) (*registry.ChainRecord, error) {
	panic("unimplemented")
}
