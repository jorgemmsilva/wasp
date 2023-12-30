package corecontracts

import (
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/vm/core/root"
)

func GetContractRecords(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) (map[isc.Hname]*root.ContractRecord, error) {
	_, recs, err := callViewInvoker(root.ViewGetContractRecords.Message(), blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}
	return root.ViewGetContractRecords.Output.Decode(recs)
}
