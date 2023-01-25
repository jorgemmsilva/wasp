package tests

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/wasp/client/chainclient"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/tools/cluster"
)

// TODO remove this?
func setupWithNoChain(t *testing.T, opt ...waspClusterOpts) *chainEnv {
	return &chainEnv{t: t, Clu: newCluster(t, opt...)}
}

type chainEnv struct {
	t     *testing.T
	Clu   *cluster.Cluster
	Chain *cluster.Chain
}

func newChainEnv(t *testing.T, clu *cluster.Cluster, chain *cluster.Chain) *chainEnv {
	return &chainEnv{
		t:     t,
		Clu:   clu,
		Chain: chain,
	}
}

func (e *chainEnv) deployWasmContract(wasmName, scDescription string, initParams map[string]interface{}) hashing.HashValue {
	wasmPath := "wasm/" + wasmName + "_bg.wasm"

	wasmBin, err := os.ReadFile(wasmPath)
	require.NoError(e.t, err)
	chClient := chainclient.New(e.Clu.L1Client(), e.Clu.WaspClient(0), e.Chain.ChainID, e.Chain.OriginatorKeyPair)

	reqTx, err := chClient.DepositFunds(1_000_000)
	require.NoError(e.t, err)
	_, err = e.Chain.CommitteeMultiClient().WaitUntilAllRequestsProcessedSuccessfully(e.Chain.ChainID, reqTx, 30*time.Second)
	require.NoError(e.t, err)

	ph, err := e.Chain.DeployWasmContract(wasmName, scDescription, wasmBin, initParams)
	require.NoError(e.t, err)
	e.t.Logf("deployContract: proghash = %s\n", ph.String())
	return ph
}

func SetupWithChain(t *testing.T, opt ...waspClusterOpts) *chainEnv {
	e := setupWithNoChain(t, opt...)
	chain, err := e.Clu.DeployDefaultChain()
	require.NoError(t, err)
	return newChainEnv(e.t, e.Clu, chain)
}

func (e *chainEnv) NewChainClient() *chainclient.Client {
	wallet, _, err := e.Clu.NewKeyPairWithFunds()
	require.NoError(e.t, err)
	return chainclient.New(e.Clu.L1Client(), e.Clu.WaspClient(0), e.Chain.ChainID, wallet)
}

func (e *chainEnv) DepositFunds(amount uint64, keyPair *cryptolib.KeyPair) {
	accountsClient := e.Chain.SCClient(accounts.Contract.Hname(), keyPair)
	tx, err := accountsClient.PostRequest(accounts.FuncDeposit.Name, chainclient.PostRequestParams{
		Transfer: isc.NewFungibleBaseTokens(amount),
	})
	require.NoError(e.t, err)
	_, err = e.Chain.CommitteeMultiClient().WaitUntilAllRequestsProcessedSuccessfully(e.Chain.ChainID, tx, 30*time.Second)
	require.NoError(e.t, err)
}
