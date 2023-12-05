package corecontracts

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
)

func GetAllowedStateControllerAddresses(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) ([]iotago.Address, error) {
	_, res, err := callViewInvoker(governance.Contract.Hname(), governance.ViewGetAllowedStateControllerAddresses.Hname(), nil, blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}

	addresses := collections.NewArrayReadOnly(res, governance.ParamAllowedStateControllerAddresses)
	ret := make([]iotago.Address, addresses.Len())
	for i := range ret {
		ret[i], err = codec.DecodeAddress(addresses.GetAt(uint32(i)))
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func GetChainOwner(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) (isc.AgentID, error) {
	_, ret, err := callViewInvoker(governance.Contract.Hname(), governance.ViewGetChainOwner.Hname(), nil, blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}

	ownerBytes := ret.Get(governance.ParamChainOwner)
	ownerID, err := isc.AgentIDFromBytes(ownerBytes)
	if err != nil {
		return nil, err
	}

	return ownerID, nil
}

func GetChainInfo(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) (*isc.ChainInfo, error) {
	chainID, ret, err := callViewInvoker(governance.Contract.Hname(), governance.ViewGetChainInfo.Hname(), nil, blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}

	var chainInfo *isc.ChainInfo

	if chainInfo, err = governance.GetChainInfo(ret, chainID); err != nil {
		return nil, err
	}

	return chainInfo, nil
}
