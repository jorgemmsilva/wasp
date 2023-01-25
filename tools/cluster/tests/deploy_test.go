package tests

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/vm/core/corecontracts"
	"github.com/iotaledger/wasp/packages/vm/core/root"
)

// executed in cluster_test.go
func testDeployChain(t *testing.T, e *chainEnv) {
	chainID, chainOwnerID := e.getChainInfo()
	require.EqualValues(t, chainID, e.Chain.ChainID)
	require.EqualValues(t, chainOwnerID, isc.NewAgentID(e.Chain.OriginatorAddress()))
	t.Logf("--- chainID: %s", chainID.String())
	t.Logf("--- chainOwnerID: %s", chainOwnerID.String())

	e.checkCoreContracts()
	e.checkRootsOutside()
	for _, i := range e.Chain.CommitteeNodes {
		blockIndex, err := e.Chain.BlockIndex(i)
		require.NoError(t, err)
		require.Greater(t, blockIndex, uint32(1))

		contractRegistry, err := e.Chain.ContractRegistry(i)
		require.NoError(t, err)
		require.EqualValues(t, len(corecontracts.All), len(contractRegistry))
	}
}

// executed in cluster_test.go
func testDeployContractOnly(t *testing.T, e *chainEnv) {
	e.deployWasmInccounter(0)

	// test calling root.FuncFindContractByName view function using client
	ret, err := e.Chain.Cluster.WaspClient(0).CallView(
		e.Chain.ChainID, root.Contract.Hname(), root.ViewFindContract.Name,
		dict.Dict{
			root.ParamHname: incHname.Bytes(),
		})
	require.NoError(t, err)
	recb, err := ret.Get(root.ParamContractRecData)
	require.NoError(t, err)
	rec, err := root.ContractRecordFromBytes(recb)
	require.NoError(t, err)
	require.EqualValues(t, "testing contract deployment with inccounter", rec.Description)
}
