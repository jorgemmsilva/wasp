package sbtests

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/vm/core/corecontracts"
	"github.com/iotaledger/wasp/packages/vm/core/testcore/sbtests/sbtestsc"
)

func TestInitSuccess(t *testing.T) {
	_, chain := setupChain(t, nil)
	err := chain.DeployContract(nil, ScName, sbtestsc.Contract.ProgramHash, nil)
	require.NoError(t, err)
}

func TestInitFail(t *testing.T) {
	_, chain := setupChain(t, nil)
	err := chain.DeployContract(nil, ScName, sbtestsc.Contract.ProgramHash, dict.Dict{
		sbtestsc.ParamFail: []byte{1},
	})
	require.Error(t, err)
}

func TestInitFailRepeat(t *testing.T) {
	_, chain := setupChain(t, nil)
	err := chain.DeployContract(nil, ScName, sbtestsc.Contract.ProgramHash, dict.Dict{
		sbtestsc.ParamFail: []byte{1},
	})
	require.Error(t, err)
	_, _, rec := chain.GetInfo()
	require.EqualValues(t, len(corecontracts.All), len(rec))

	// repeat must succeed
	err = chain.DeployContract(nil, ScName, sbtestsc.Contract.ProgramHash, nil)
	require.NoError(t, err)
	_, _, rec = chain.GetInfo()
	require.EqualValues(t, len(corecontracts.All)+1, len(rec))
}

func TestInitFailRepeatWasm(t *testing.T) {
	if forceSkipWasm {
		t.SkipNow()
	}
	_, chain := setupChain(t, nil)
	err := chain.DeployWasmContract(nil, ScName, WasmFileTestcore, dict.Dict{
		sbtestsc.ParamFail: []byte{1},
	})
	require.Error(t, err)
	_, _, rec := chain.GetInfo()
	require.EqualValues(t, len(corecontracts.All), len(rec))

	// repeat must succeed
	err = chain.DeployWasmContract(nil, ScName, WasmFileTestcore, nil)
	require.NoError(t, err)
	_, _, rec = chain.GetInfo()
	require.EqualValues(t, len(corecontracts.All)+1, len(rec))
}

func TestInitSuccess2(t *testing.T) {
	_, chain := setupChain(t, nil)
	err := chain.DeployContract(nil, ScName, sbtestsc.Contract.ProgramHash, nil)
	require.NoError(t, err)
}
