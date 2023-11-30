package corecontracts

import (
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/vm/core/root"
)

func GetContractRecords(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) (map[isc.Hname]*root.ContractRecord, error) {
	_, recs, err := callViewInvoker(root.Contract.Hname(), root.ViewGetContractRecords.Hname(), nil, blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}

	contracts, err := root.DecodeContractRegistry(collections.NewMapReadOnly(recs, root.VarContractRegistry))
	if err != nil {
		return nil, err
	}

	return contracts, nil
}
