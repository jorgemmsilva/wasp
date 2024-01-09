package corecontracts

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
)

func GetAllowedStateControllerAddresses(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) ([]iotago.Address, error) {
	_, res, err := callViewInvoker(governance.ViewGetAllowedStateControllerAddresses.Message(), blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}
	return governance.ViewGetAllowedStateControllerAddresses.Output.Decode(res)
}

func GetChainOwner(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) (isc.AgentID, error) {
	_, ret, err := callViewInvoker(governance.ViewGetChainOwner.Message(), blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}
	return governance.ViewGetChainOwner.Output.Decode(ret)
}

func GetChainInfo(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) (*isc.ChainInfo, error) {
	_, ret, err := callViewInvoker(governance.ViewGetChainInfo.Message(), blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}
	return governance.ViewGetChainInfo.Output.Decode(ret)
}
