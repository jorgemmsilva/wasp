package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/wasp/client/chainclient"
	"github.com/iotaledger/wasp/client/scclient"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/utxodb"
	"github.com/iotaledger/wasp/tools/cluster/templates"
)

// executed in cluster_test.go
func testPermitionlessAccessNode(t *testing.T, e *chainEnv) {
	// deploy the inccounter for the test to use
	e.deployWasmInccounter(0)

	// deposit funds for offledger requests
	keyPair, _, err := e.Clu.NewKeyPairWithFunds()
	require.NoError(t, err)

	e.DepositFunds(utxodb.FundsFromFaucetAmount, keyPair)

	// spin a new node
	clu2 := newCluster(t, waspClusterOpts{
		nNodes:  1,
		dirName: "wasp-cluster-access-node",
		modifyConfig: func(nodeIndex int, configParams templates.WaspConfigParams) templates.WaspConfigParams {
			// avoid port conflicts when running everything on localhost
			configParams.APIPort += 100
			configParams.DashboardPort += 100
			configParams.MetricsPort += 100
			configParams.NanomsgPort += 100
			configParams.PeeringPort += 100
			configParams.ProfilingPort += 100
			return configParams
		},
	})
	// remove this cluster when the test ends
	t.Cleanup(clu2.Stop)

	nodeClient := e.Clu.WaspClient(0)
	accessNodeClient := clu2.WaspClient(0)

	// adds node #0 from cluster2 as access node of node #0 from cluster1

	// trust setup between the two nodes
	node0peerInfo, err := nodeClient.GetPeeringSelf()
	require.NoError(t, err)
	err = clu2.AddTrustedNode(node0peerInfo)
	require.NoError(t, err)

	accessNodePeerInfo, err := accessNodeClient.GetPeeringSelf()
	require.NoError(t, err)
	err = e.Clu.AddTrustedNode(accessNodePeerInfo, []int{0})
	require.NoError(t, err)

	// activate the chain on the access node
	err = accessNodeClient.PutChainRecord(registry.NewChainRecord(e.Chain.ChainID, true, []*cryptolib.PublicKey{}))
	require.NoError(t, err)

	// add node 0 from cluster 2 as a *permitionless* access node
	err = nodeClient.AddAccessNode(e.Chain.ChainID, accessNodePeerInfo.PubKey)
	require.NoError(t, err)

	// give some time for the access node to sync
	time.Sleep(2 * time.Second)

	// send a request to the access node
	myClient := scclient.New(
		chainclient.New(
			e.Clu.L1Client(),
			accessNodeClient,
			e.Chain.ChainID,
			keyPair,
		),
		incHname,
	)
	req, err := myClient.PostOffLedgerRequest(incrementFuncName)
	require.NoError(t, err)

	// request has been processed
	_, err = e.Chain.CommitteeMultiClient().WaitUntilRequestProcessedSuccessfully(e.Chain.ChainID, req.ID(), 1*time.Minute)
	require.NoError(t, err)

	// remove the access node from cluster1 node 0
	err = nodeClient.RemoveAccessNode(e.Chain.ChainID, accessNodePeerInfo.PubKey)
	require.NoError(t, err)

	// try sending the request again
	req, err = myClient.PostOffLedgerRequest(incrementFuncName)
	require.NoError(t, err)

	// request is not processed after a while
	time.Sleep(2 * time.Second)
	rec, err := nodeClient.RequestReceipt(e.Chain.ChainID, req.ID())
	require.Error(t, err)
	require.Regexp(t, `"Code":404`, err.Error())
	require.Nil(t, rec)
}
