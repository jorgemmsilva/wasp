package tests

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/wasp/client/scclient"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
)

const (
	incName        = "inccounter"
	incDescription = "IncCounter, a PoC smart contract"
)

var (
	incHname = isc.Hn(incName)

	incrementFuncName = "increment"
	incrementFuncHn   = isc.Hn(incrementFuncName)

	incrementRepeatManyFuncName = "repeatMany"
)

const (
	varCounter         = "counter"
	varNumRepeats      = "numRepeats"
	varDelay           = "delay"
	getCounterViewName = "getCounter"
)

func (e *chainEnv) deployWasmInccounter(initialCounter int64) hashing.HashValue {
	initParams := make(map[string]interface{})
	initParams[varCounter] = initialCounter
	return e.deployWasmContract(incName, incDescription, initParams)
}

// TODO rename
func (e *chainEnv) GetCounterValue(nodeIndex ...int) int64 {
	cl := e.Chain.SCClient(incHname, nil, nodeIndex...)
	ret, err := cl.CallView(getCounterViewName, nil)
	require.NoError(e.t, err)
	counter, err := codec.DecodeInt64(ret.MustGet(varCounter), 0)
	require.NoError(e.t, err)
	return counter
}

func (e *chainEnv) expectCounter(counter int64, nodeIndex ...int) {
	require.EqualValues(e.t, counter, e.GetCounterValue(nodeIndex...))
}

func (e *chainEnv) newInccounterClientWithFunds() *scclient.SCClient {
	keyPair, _, err := e.Clu.NewKeyPairWithFunds()
	require.NoError(e.t, err)
	return e.Chain.SCClient(incHname, keyPair)
}

// TODO rename
func (e *chainEnv) counterEquals(expected int64) conditionFn {
	return func(t *testing.T, nodeIndex int) bool {
		ret, err := e.Chain.Cluster.WaspClient(nodeIndex).CallView(
			e.Chain.ChainID, incHname, getCounterViewName, nil,
		)
		if err != nil {
			e.t.Logf("chainEnv::counterEquals: failed to call GetCounter: %v", err)
			return false
		}
		counter, err := codec.DecodeInt64(ret.MustGet(varCounter), 0)
		require.NoError(t, err)
		t.Logf("chainEnv::counterEquals: node %d: counter: %d, waiting for: %d", nodeIndex, counter, expected)
		return counter == expected
	}
}
