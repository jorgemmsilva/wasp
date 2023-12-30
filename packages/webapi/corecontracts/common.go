package corecontracts

import (
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/webapi/common"
)

type CallViewInvoker func(msg isc.Message, blockIndexOrHash string) (isc.ChainID, dict.Dict, error)

func MakeCallViewInvoker(ch chaintypes.Chain) CallViewInvoker {
	return func(msg isc.Message, blockIndexOrHash string) (isc.ChainID, dict.Dict, error) {
		ret, err := common.CallView(ch, msg, blockIndexOrHash)
		return ch.ID(), ret, err
	}
}
