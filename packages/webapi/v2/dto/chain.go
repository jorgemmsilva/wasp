package dto

import (
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

type (
	ContractsMap map[isc.Hname]*root.ContractRecord
)

type ChainInfo struct {
	IsActive        bool
	ChainID         isc.ChainID
	ChainOwnerID    isc.AgentID
	Description     string
	GasFeePolicy    *gas.GasFeePolicy
	MaxBlobSize     uint32
	MaxEventSize    uint16
	MaxEventsPerReq uint16
}

func MapChainInfo(info *governance.ChainInfo, isActive bool) *ChainInfo {
	chID, err := info.ChainIDDeserialized()
	if err != nil {
		panic(err)
	}
	chOwner, err := info.ChainOwnerIDDeserialized()
	if err != nil {
		panic(err)
	}
	feePolicy, err := info.GasFeePolicyDeserialized()
	if err != nil {
		panic(err)
	}
	return &ChainInfo{
		IsActive:        isActive,
		ChainID:         chID,
		ChainOwnerID:    chOwner,
		Description:     info.Description,
		GasFeePolicy:    feePolicy,
		MaxBlobSize:     info.MaxBlobSize,
		MaxEventSize:    info.MaxEventSize,
		MaxEventsPerReq: info.MaxEventsPerReq,
	}
}
