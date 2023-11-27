package corecontracts

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/webapi/common"
)

type CallViewInvoker func(contractName isc.Hname, functionName isc.Hname, params dict.Dict, blockIndexOrHash string) (isc.ChainID, dict.Dict, error)

func MakeCallViewInvoker(ch chaintypes.Chain, l1API iotago.API, baseTokenInfo api.InfoResBaseToken) CallViewInvoker {
	return func(contractName isc.Hname, functionName isc.Hname, params dict.Dict, blockIndexOrHash string) (isc.ChainID, dict.Dict, error) {
		ret, err := common.CallView(ch, l1API, baseTokenInfo, accounts.Contract.Hname(), accounts.ViewAccounts.Hname(), nil, blockIndexOrHash)
		return ch.ID(), ret, err
	}
}
