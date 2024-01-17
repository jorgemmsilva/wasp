// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package governanceimpl

import (
	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/kv/kvdecoder"
	"github.com/iotaledger/wasp/packages/vm/core/errors/coreerrors"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
)

var errOwnerNotDelegated = coreerrors.Register("not delegated to another chain owner").Create()

// claimChainOwnership changes the chain owner to the delegated agentID (if any)
// Checks authorization if the caller is the one to which the ownership is delegated
// Note that ownership is only changed by the successful call to  claimChainOwnership
func claimChainOwnership(ctx isc.Sandbox) dict.Dict {
	ctx.Log().LogDebugf("governance.delegateChainOwnership.begin")
	state := ctx.State()

	stateDecoder := kvdecoder.New(state, ctx.Log())
	currentOwner := stateDecoder.MustGetAgentID(governance.VarChainOwnerID)
	nextOwner := stateDecoder.MustGetAgentID(governance.VarChainOwnerIDDelegated, currentOwner)

	if nextOwner.Equals(currentOwner) {
		panic(errOwnerNotDelegated)
	}
	ctx.RequireCaller(nextOwner)

	state.Set(governance.VarChainOwnerID, codec.AgentID.Encode(nextOwner))
	state.Del(governance.VarChainOwnerIDDelegated)
	ctx.Log().LogDebugf("governance.chainChainOwner.success: chain owner changed: %s --> %s",
		currentOwner.String(),
		nextOwner.String(),
	)
	return nil
}

// delegateChainOwnership stores next possible (delegated) chain owner to another agentID
// checks authorization by the current owner
// Two-step process allow/change is in order to avoid mistakes
func delegateChainOwnership(ctx isc.Sandbox, newOwnerID isc.AgentID) dict.Dict {
	ctx.Log().LogDebugf("governance.delegateChainOwnership.begin")
	ctx.RequireCallerIsChainOwner()
	ctx.State().Set(governance.VarChainOwnerIDDelegated, codec.AgentID.Encode(newOwnerID))
	ctx.Log().LogDebugf("governance.delegateChainOwnership.success: chain ownership delegated to %s", newOwnerID.String())
	return nil
}

func setPayoutAgentID(ctx isc.Sandbox, agent isc.AgentID) dict.Dict {
	ctx.RequireCallerIsChainOwner()
	ctx.State().Set(governance.VarPayoutAgentID, codec.AgentID.Encode(agent))
	return nil
}

func getPayoutAgentID(ctx isc.SandboxView) isc.AgentID {
	return lo.Must(codec.AgentID.Decode(ctx.StateR().Get(governance.VarPayoutAgentID)))
}

func setMinCommonAccountBalance(ctx isc.Sandbox, minCommonAccountBalance iotago.BaseToken) dict.Dict {
	ctx.RequireCallerIsChainOwner()
	ctx.State().Set(governance.VarMinBaseTokensOnCommonAccount, codec.BaseToken.Encode(minCommonAccountBalance))
	return nil
}

func getMinCommonAccountBalance(ctx isc.SandboxView) iotago.BaseToken {
	return lo.Must(codec.BaseToken.Decode(ctx.StateR().Get(governance.VarMinBaseTokensOnCommonAccount)))
}

func getChainOwner(ctx isc.SandboxView) isc.AgentID {
	return lo.Must(codec.AgentID.Decode(ctx.StateR().Get(governance.VarChainOwnerID)))
}
