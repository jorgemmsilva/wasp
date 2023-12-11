package chainutil

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
)

func GetAccountBalance(
	ch chaintypes.ChainCore,
	agentID isc.AgentID,
	l1API iotago.API,
	tokenInfo api.InfoResBaseToken,
) (*isc.FungibleTokens, error) {
	params := codec.MakeDict(map[string]interface{}{
		accounts.ParamAgentID: codec.EncodeAgentID(agentID),
	})
	ret, err := CallView(mustLatestState(ch), ch, accounts.Contract.Hname(), accounts.ViewBalance.Hname(), params)
	if err != nil {
		return nil, err
	}
	return isc.FungibleTokensFromDict(ret)
}

func mustLatestState(ch chaintypes.ChainCore) state.State {
	latestState, err := ch.LatestState(chaintypes.ActiveOrCommittedState)
	if err != nil {
		panic(err)
	}
	return latestState
}
