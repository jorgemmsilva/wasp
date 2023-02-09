package dashboard

import (
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/root"
)

type ChainInfo struct {
	*governance.ChainInfo
	Contracts map[isc.Hname]*root.ContractRecord
}

func (d *Dashboard) fetchChainInfo(chainID isc.ChainID) (ret *ChainInfo, err error) {
	data, err := d.wasp.CallView(chainID, governance.Contract.Name, governance.ViewGetChainInfo.Name, nil)
	if err != nil {
		d.log.Error(err)
		return
	}

	ret.ChainInfo, err = util.Deserialize[*governance.ChainInfo](data)
	if err != nil {
		d.log.Error(err)
		return
	}

	recsData, err := d.wasp.CallView(chainID, root.Contract.Name, root.ViewGetContractRecords.Name, nil)
	if err != nil {
		return
	}

	ret.Contracts, err = root.DecodeContractRegistry(recsData)
	if err != nil {
		return
	}

	return ret, err
}
