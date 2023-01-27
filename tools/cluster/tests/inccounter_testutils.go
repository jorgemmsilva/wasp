package tests

import (
	"testing"
	"time"

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

func (e *chainEnv) getCounterValue(nodeIndex ...int) int64 {
	cl := e.Chain.SCClient(incHname, nil, nodeIndex...)
	ret, err := cl.CallView(getCounterViewName, nil)
	require.NoError(e.t, err)
	counter, err := codec.DecodeInt64(ret.MustGet(varCounter), 0)
	require.NoError(e.t, err)
	return counter
}

func (e *chainEnv) expectCounter(counter int64, nodeIndex ...int) {
	require.EqualValues(e.t, counter, e.getCounterValue(nodeIndex...))
}

func (e *chainEnv) newInccounterClientWithFunds() *scclient.SCClient {
	keyPair, _, err := e.Clu.NewKeyPairWithFunds()
	require.NoError(e.t, err)
	return e.Chain.SCClient(incHname, keyPair)
}

func (e *chainEnv) counterEqualsCondition(expected int64) conditionFn {
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

func (e *chainEnv) waitUntilCounterEquals(hname isc.Hname, expected int64, duration time.Duration) {
	timeout := time.After(duration)
	var c int64
	allNodesEqualFun := func() bool {
		for _, node := range e.Chain.AllPeers {
			c = e.getCounterValue(node)
			if c != expected {
				return false
			}
		}
		return true
	}
	for {
		select {
		case <-timeout:
			e.t.Errorf("timeout waiting for inccounter, current: %d, expected: %d", c, expected)
			e.t.Fatal()
		default:
			if allNodesEqualFun() {
				return // success
			}
		}
		time.Sleep(1 * time.Second)
	}
}
