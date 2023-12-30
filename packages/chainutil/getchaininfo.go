package chainutil

import (
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
)

func GetAccountBalance(ch chaintypes.ChainCore, agentID isc.AgentID) (*isc.FungibleTokens, error) {
	ret, err := CallView(mustLatestState(ch), ch, accounts.ViewBalance.Message(agentID))
	if err != nil {
		return nil, err
	}
	return accounts.ViewBalance.Output.Decode(ret)
}

func mustLatestState(ch chaintypes.ChainCore) state.State {
	latestState, err := ch.LatestState(chaintypes.ActiveOrCommittedState)
	if err != nil {
		panic(err)
	}
	return latestState
}
