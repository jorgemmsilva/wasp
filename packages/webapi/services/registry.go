package services

import (
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/webapi/interfaces"
)

type RegistryService struct {
	chainsProvider              chaintypes.ChainsProvider
	chainRecordRegistryProvider registry.ChainRecordRegistryProvider
}

func NewRegistryService(chainsProvider chaintypes.ChainsProvider, chainRecordRegistryProvider registry.ChainRecordRegistryProvider) interfaces.RegistryService {
	return &RegistryService{
		chainsProvider:              chainsProvider,
		chainRecordRegistryProvider: chainRecordRegistryProvider,
	}
}

func (c *RegistryService) GetChainRecordByChainID(chainID isc.ChainID) (*registry.ChainRecord, error) {
	return c.chainRecordRegistryProvider.ChainRecord(chainID)
}
