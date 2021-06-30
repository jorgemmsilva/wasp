package sbtests

import (
	"testing"
	"time"

	"github.com/iotaledger/wasp/packages/coretypes"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/vm/core"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/testcore/sbtests/sbtestsc"
	"github.com/stretchr/testify/require"
)

func Test2Chains(t *testing.T) { run2(t, test2Chains) }
func test2Chains(t *testing.T, w bool) {
	core.PrintWellKnownHnames()

	env := solo.New(t, false, false).WithNativeContract(sbtestsc.Interface)
	chain1 := env.NewChain(nil, "ch1")
	chain2 := env.NewChain(nil, "ch2")
	chain1.CheckAccountLedger()
	chain2.CheckAccountLedger()

	contractAgentID1, extraToken1 := setupTestSandboxSC(t, chain1, nil, w)
	contractAgentID2, extraToken2 := setupTestSandboxSC(t, chain2, nil, w)

	userWallet, userAddress := env.NewKeyPairWithFunds()
	userAgentID := coretypes.NewAgentID(userAddress, 0)
	env.AssertAddressIotas(userAddress, solo.Saldo)

	chain1.AssertIotas(contractAgentID1, 1)
	chain1.AssertIotas(contractAgentID2, 0)
	chain1.AssertCommonAccountIotas(2 + extraToken1)
	chain1.AssertTotalIotas(3 + extraToken1)

	chain2.AssertIotas(contractAgentID1, 0)
	chain2.AssertIotas(contractAgentID2, 1)
	chain2.AssertCommonAccountIotas(2 + extraToken2)
	chain2.AssertTotalIotas(3 + extraToken2)

	req := solo.NewCallParams(accounts.Interface.Name, accounts.FuncDeposit,
		accounts.ParamAgentID, contractAgentID2,
	).WithIotas(42)
	_, err := chain1.PostRequestSync(req, userWallet)
	require.NoError(t, err)

	env.AssertAddressIotas(userAddress, solo.Saldo-42)

	chain1.AssertIotas(userAgentID, 0)
	chain1.AssertIotas(contractAgentID1, 1)
	chain1.AssertIotas(contractAgentID2, 42)
	chain1.AssertCommonAccountIotas(2 + extraToken1)
	chain1.AssertTotalIotas(45 + extraToken1)

	chain2.AssertIotas(userAgentID, 0)
	chain2.AssertIotas(contractAgentID1, 0)
	chain2.AssertIotas(contractAgentID2, 1)
	chain2.AssertCommonAccountIotas(2 + extraToken2)
	chain2.AssertTotalIotas(3 + extraToken2)

	req = solo.NewCallParams(ScName, sbtestsc.FuncWithdrawToChain,
		sbtestsc.ParamChainID, chain1.ChainID,
	).WithIotas(1)

	_, err = chain2.PostRequestSync(req, userWallet)
	require.NoError(t, err)

	extra := 0
	if w {
		extra = 1
	}
	require.True(t, chain1.WaitForRequestsThrough(5+extra, 10*time.Second))
	require.True(t, chain2.WaitForRequestsThrough(5+extra, 10*time.Second))

	env.AssertAddressIotas(userAddress, solo.Saldo-42-1)

	chain1.AssertIotas(userAgentID, 0)
	chain1.AssertIotas(contractAgentID1, 1)
	chain1.AssertIotas(contractAgentID2, 0)
	chain1.AssertCommonAccountIotas(3 + extraToken1)
	chain1.AssertTotalIotas(4 + extraToken1)

	chain2.AssertIotas(userAgentID, 0)
	chain2.AssertIotas(contractAgentID1, 0)
	chain2.AssertIotas(contractAgentID2, 43)
	chain2.AssertCommonAccountIotas(2 + extraToken2)
	chain2.AssertTotalIotas(45 + extraToken2)
}
