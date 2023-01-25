package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/wasp/client/chainclient"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/utxodb"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
)

// executed in cluster_test.go
func testBasicAccounts(t *testing.T, e *chainEnv) {
	testAccounts(e)
}

func TestBasicAccountsNLow(t *testing.T) {
	runTest := func(tt *testing.T, n, t int) {
		e := setupWithNoChain(tt)
		chainNodes := make([]int, n)
		for i := range chainNodes {
			chainNodes[i] = i
		}
		chain, err := e.Clu.DeployChainWithDKG(fmt.Sprintf("low_node_chain_%v_%v", n, t), chainNodes, chainNodes, uint16(t))
		require.NoError(tt, err)
		env := newChainEnv(tt, e.Clu, chain)
		testAccounts(env)
	}
	t.Run("N=1", func(tt *testing.T) { runTest(tt, 1, 1) })
	t.Run("N=2", func(tt *testing.T) { runTest(tt, 2, 2) })
	t.Run("N=3", func(tt *testing.T) { runTest(tt, 3, 3) })
	t.Run("N=4", func(tt *testing.T) { runTest(tt, 4, 3) })
}

func testAccounts(e *chainEnv) {
	e.deployWasmInccounter(42)

	myWallet, myAddress, err := e.Clu.NewKeyPairWithFunds()
	require.NoError(e.t, err)

	transferBaseTokens := 1 * isc.Million
	chClient := chainclient.New(e.Clu.L1Client(), e.Clu.WaspClient(0), e.Chain.ChainID, myWallet)

	par := chainclient.NewPostRequestParams().WithBaseTokens(transferBaseTokens)
	reqTx, err := chClient.Post1Request(incHname, incrementFuncHn, *par)
	require.NoError(e.t, err)

	receipts, err := e.Chain.CommitteeMultiClient().WaitUntilAllRequestsProcessedSuccessfully(e.Chain.ChainID, reqTx, 10*time.Second)
	require.NoError(e.t, err)

	fees := receipts[0].GasFeeCharged
	e.checkBalanceOnChain(isc.NewAgentID(myAddress), isc.BaseTokenID, transferBaseTokens-fees)

	for i := range e.Chain.CommitteeNodes {
		e.expectCounter(43, i)
	}

	if !e.Clu.AssertAddressBalances(myAddress, isc.NewFungibleBaseTokens(utxodb.FundsFromFaucetAmount-transferBaseTokens)) {
		e.t.Fatal()
	}

	incCounterAgentID := isc.NewContractAgentID(e.Chain.ChainID, incHname)
	e.checkBalanceOnChain(incCounterAgentID, isc.BaseTokenID, 0)
}

// executed in cluster_test.go
func testBasic2Accounts(t *testing.T, e *chainEnv) {
	e.deployWasmInccounter(42)
	originatorSigScheme := e.Chain.OriginatorKeyPair
	originatorAddress := e.Chain.OriginatorAddress()

	e.checkLedger()

	myWallet, myAddress, err := e.Clu.NewKeyPairWithFunds()
	require.NoError(t, err)

	transferBaseTokens := 1 * isc.Million
	myWalletClient := chainclient.New(e.Clu.L1Client(), e.Clu.WaspClient(0), e.Chain.ChainID, myWallet)

	par := chainclient.NewPostRequestParams().WithBaseTokens(transferBaseTokens)
	reqTx, err := myWalletClient.Post1Request(incHname, incrementFuncHn, *par)
	require.NoError(t, err)

	_, err = e.Chain.CommitteeMultiClient().WaitUntilAllRequestsProcessedSuccessfully(e.Chain.ChainID, reqTx, 30*time.Second)
	require.NoError(t, err)
	e.checkLedger()

	for _, i := range e.Chain.CommitteeNodes {
		e.expectCounter(43, i)
	}
	if !e.Clu.AssertAddressBalances(myAddress, isc.NewFungibleBaseTokens(utxodb.FundsFromFaucetAmount-transferBaseTokens)) {
		t.Fatal()
	}

	e.printAccounts("withdraw before")

	// withdraw back 500 base tokens to originator address
	fmt.Printf("\norig address from sigsheme: %s\n", originatorAddress.Bech32(parameters.L1().Protocol.Bech32HRP))
	origL1Balance := e.Clu.AddressBalances(originatorAddress).BaseTokens
	originatorClient := chainclient.New(e.Clu.L1Client(), e.Clu.WaspClient(0), e.Chain.ChainID, originatorSigScheme)
	allowanceBaseTokens := uint64(800_000)
	req2, err := originatorClient.PostOffLedgerRequest(accounts.Contract.Hname(), accounts.FuncWithdraw.Hname(),
		chainclient.PostRequestParams{
			Allowance: isc.NewAllowanceBaseTokens(allowanceBaseTokens),
		},
	)
	require.NoError(t, err)

	_, err = e.Chain.CommitteeMultiClient().WaitUntilRequestProcessedSuccessfully(e.Chain.ChainID, req2.ID(), 30*time.Second)
	require.NoError(t, err)

	e.checkLedger()

	e.printAccounts("withdraw after")

	require.Equal(t, e.Clu.AddressBalances(originatorAddress).BaseTokens, origL1Balance+allowanceBaseTokens)
}
