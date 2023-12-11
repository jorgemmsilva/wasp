package corecontracts

import (
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/webapi/common"
)

type CallViewInvoker func(contractName isc.Hname, functionName isc.Hname, params dict.Dict, blockIndexOrHash string) (isc.ChainID, dict.Dict, error)

func MakeCallViewInvoker(ch chaintypes.Chain) CallViewInvoker {
	return func(contractName isc.Hname, functionName isc.Hname, params dict.Dict, blockIndexOrHash string) (isc.ChainID, dict.Dict, error) {
		ret, err := common.CallView(ch, accounts.Contract.Hname(), accounts.ViewAccounts.Hname(), nil, blockIndexOrHash)
		return ch.ID(), ret, err
	}
}
