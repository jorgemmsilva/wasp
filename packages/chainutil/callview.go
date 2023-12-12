package chainutil

import (
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
) (dict.Dict, error) {
	vctx, err := viewcontext.New(ch, chainState, false)
	if err != nil {
		return nil, err
	}
	return vctx.CallViewExternal(contractHname, viewHname, params)
}
