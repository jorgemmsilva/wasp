// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package wasmsolo

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
)

const Million = uint64(1_000_000)

type SoloBalances struct {
	Account    uint64
	accounts   map[string]uint64
	agents     []*SoloAgent
	Common     uint64
	ctx        *SoloContext
	Originator uint64
}

// NewSoloBalances takes a snapshot of all balances necessary to track token
// movements easily. It will track L2 Originator, Common, snd SC Account balances
// Additional agents can be specified as extra accounts
// This is typically called from SoloContext.Balances() before a call to the SC.
// After the call, update the balances with the expected token movements and then
// call SoloBalances.VerifyBalances() to see if they match the actual balances.
func NewSoloBalances(ctx *SoloContext, agents ...*SoloAgent) *SoloBalances {
	bal := &SoloBalances{
		ctx:        ctx,
		Account:    ctx.Balance(ctx.Account()),
		Common:     ctx.Balance(ctx.CommonAccount()),
		Originator: ctx.Balance(ctx.Originator()),
		agents:     agents,
		accounts:   make(map[string]uint64),
	}
	for _, agent := range agents {
		bal.accounts[agent.AgentID().String()] = ctx.Balance(agent)
	}
	bal.DumpBalances()
	return bal
}

func (bal *SoloBalances) Add(agent *SoloAgent, balance uint64) {
	bal.accounts[agent.AgentID().String()] += balance
}

// DumpBalances prints all known accounts, both L2 and L1, in debug mode.
// It uses the L2 ledger to enumerate the known accounts.
func (bal *SoloBalances) DumpBalances() {
	if !SoloDebug {
		return
	}
	ctx := bal.ctx
	accs := ctx.Chain.L2Accounts()
	for _, agent := range bal.agents {
		agentID := agent.AgentID()
		if !contains(accs, agentID) {
			accs = append(accs, agentID)
		}
	}
	sort.Slice(accs, func(i, j int) bool {
		return bal.findName(accs[i].String()) < bal.findName(accs[j].String())
	})
	txt := "ACCOUNTS:"
	for _, acc := range accs {
		l2 := ctx.Chain.L2Assets(acc)
		addr, ok := isc.AddressFromAgentID(acc)
		l1 := isc.NewEmptyAssets()
		if ok {
			l1 = ctx.Chain.Env.L1Assets(addr)
		}
		id := acc.String()
		txt += fmt.Sprintf("\n%-19s %s\n\tL2: %10d", bal.findName(id), id, l2.BaseTokens)
		hname := isc.HnameFromAgentID(acc)
		if hname.IsNil() {
			txt += fmt.Sprintf(",\tL1: %10d", l1.BaseTokens)
		}
		for l2NativeTokenID, l2NativeTokenAmount := range l2.NativeTokens {
			txt += fmt.Sprintf("\n\tL2: %10d", l2NativeTokenAmount)
			tokTxt := ",\t           "
			if hname.IsNil() {
				for l1NativeTokenID := range l1.NativeTokens {
					if l1NativeTokenID == l2NativeTokenID {
						// TODO: <lmoe> Revisit this native token dump
						// l1.NativeTokens = append(l1.NativeTokens, l1.NativeTokens[i+1:]...)
						tokTxt = fmt.Sprintf(",\tL1: %10d", l1.BaseTokens)
						break
					}
				}
			}
			txt += fmt.Sprintf("%s,\t%s", tokTxt, l2NativeTokenID.String())
		}
		for nativeTokenID := range l1.NativeTokens {
			txt += fmt.Sprintf("\n\tL2: %10d,\tL1: %10d,\t%s", 0, l1.BaseTokens, nativeTokenID.String())
		}
	}
	receipt := ctx.Chain.LastReceipt()
	if receipt == nil {
		panic("DumpBalances: missing last receipt")
	}

	fmt.Printf("%s\nGas: %d, fee %d (from last receipt)\n", txt, receipt.GasBurned, receipt.GasFeeCharged)
}

func (bal *SoloBalances) findName(id string) string {
	agent := bal.ctx.Account()
	if agent.ID == id {
		return agent.Name
	}
	agent = bal.ctx.CommonAccount()
	if agent.ID == id {
		return agent.Name
	}
	agent = bal.ctx.Originator()
	if agent.ID == id {
		return agent.Name
	}
	for _, agent = range bal.agents {
		if agent.ID == id {
			return agent.Name
		}
	}
	return ""
}

func (bal *SoloBalances) UpdateFeeBalances(gasfee uint64) {
	bal.Common += gasfee
	if bal.Common > uint64(governance.DefaultMinBaseTokensOnCommonAccount) {
		exceess := bal.Common - uint64(governance.DefaultMinBaseTokensOnCommonAccount)
		bal.Common = uint64(governance.DefaultMinBaseTokensOnCommonAccount)
		bal.Originator += exceess
	}
}

func (bal *SoloBalances) VerifyBalances(t testing.TB) {
	bal.DumpBalances()
	ctx := bal.ctx
	require.EqualValues(t, bal.Account, ctx.Balance(ctx.Account()))
	require.EqualValues(t, bal.Common, ctx.Balance(ctx.CommonAccount()))
	require.EqualValues(t, bal.Originator, ctx.Balance(ctx.Originator()))
	for _, agent := range bal.agents {
		expected := bal.accounts[agent.AgentID().String()]
		require.EqualValues(t, expected, ctx.Balance(agent))
	}
}
