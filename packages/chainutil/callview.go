package chainutil

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/vm/viewcontext"
)

// CallView executes a view call on the latest block of the chain
func CallView(
	chainState state.State,
	ch chaintypes.ChainCore,
	contractHname,
	viewHname isc.Hname,
	params dict.Dict,
	l1API iotago.API,
	tokenInfo api.InfoResBaseToken,
) (dict.Dict, error) {

	vctx, err := viewcontext.New(ch, chainState, false, l1API, tokenInfo)
	if err != nil {
		return nil, err
	}
	return vctx.CallViewExternal(contractHname, viewHname, params)
}
