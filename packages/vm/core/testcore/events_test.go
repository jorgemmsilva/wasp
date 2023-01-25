package testcore

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
)

var (
	nEvents                = int(governance.DefaultMaxEventsPerRequest + 1000)
	bigEventSize           = int(governance.DefaultMaxEventSize + 1000)
	manyEventsContractName = "ManyEventsContract"
	manyEventsContract     = coreutil.NewContract(manyEventsContractName, "many events contract")

	funcManyEvents = coreutil.Func("manyevents")
	funcBigEvent   = coreutil.Func("bigevent")

	manyEventsContractProcessor = manyEventsContract.Processor(nil,
		funcManyEvents.WithHandler(func(ctx isc.Sandbox) dict.Dict {
			for i := 0; i < nEvents; i++ {
				ctx.Event(fmt.Sprintf("testing many events %d", i))
			}
			return nil
		}),
		funcBigEvent.WithHandler(func(ctx isc.Sandbox) dict.Dict {
			buf := make([]byte, bigEventSize)
			ctx.Event(string(buf))
			return nil
		}),
	)
)

func setupTest(t *testing.T) *solo.Chain {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true}).
		WithNativeContract(manyEventsContractProcessor)
	ch := env.NewChain()
	err := ch.DeployContract(nil, manyEventsContract.Name, manyEventsContract.ProgramHash)
	require.NoError(t, err)
	return ch
}

func checkNEvents(t *testing.T, ch *solo.Chain, reqid isc.RequestID, n int) {
	// fetch events from blocklog
	events := ch.GetEventsForContract(manyEventsContractName)
	require.Len(t, events, n)

	events = ch.GetEventsForRequest(reqid)
	require.Len(t, events, n)
}

func TestManyEvents(t *testing.T) {
	ch := setupTest(t)
	ch.MustDepositBaseTokensToL2(10_000_000, nil)

	// post a request that issues too many events (nEvents)
	tx, _, err := ch.PostRequestSyncTx(
		solo.NewCallParams(manyEventsContract.Name, funcManyEvents.Name).AddBaseTokens(1),
		nil,
	)
	require.Error(t, err) // error expected (too many events)
	reqs, err := ch.Env.RequestsForChain(tx, ch.ChainID)
	require.NoError(t, err)
	reqID := reqs[0].ID()
	checkNEvents(t, ch, reqID, 0) // no events are saved

	// allow for more events per request in root contract
	req := solo.NewCallParams(
		governance.Contract.Name, governance.FuncSetChainInfo.Name,
		governance.ParamMaxEventsPerRequestUint16, uint16(nEvents),
	).WithGasBudget(10_000)
	_, err = ch.PostRequestSync(req, nil)
	require.NoError(t, err)

	// check events are now saved
	req = solo.NewCallParams(manyEventsContract.Name, funcManyEvents.Name).
		WithGasBudget(10_000_000)
	tx, _, err = ch.PostRequestSyncTx(req, nil)
	require.NoError(t, err)

	reqs, err = ch.Env.RequestsForChain(tx, ch.ChainID)
	require.NoError(t, err)
	reqID = reqs[0].ID()
	checkNEvents(t, ch, reqID, nEvents)
}

func TestEventTooLarge(t *testing.T) {
	ch := setupTest(t)

	// post a request that issues an event too large
	req := solo.NewCallParams(manyEventsContract.Name, funcBigEvent.Name).
		WithGasBudget(1000)
	tx, _, err := ch.PostRequestSyncTx(req, nil)
	require.Error(t, err) // error expected (event too large)
	reqs, err := ch.Env.RequestsForChain(tx, ch.ChainID)
	require.NoError(t, err)
	reqID := reqs[0].ID()
	checkNEvents(t, ch, reqID, 0) // no events are saved

	// allow for bigger events in root contract
	req = solo.NewCallParams(
		governance.Contract.Name, governance.FuncSetChainInfo.Name,
		governance.ParamMaxEventSizeUint16, uint16(bigEventSize),
	).WithGasBudget(100_000)
	_, err = ch.PostRequestSync(req, nil)
	require.NoError(t, err)

	// check event is now saved
	req = solo.NewCallParams(manyEventsContract.Name, funcBigEvent.Name).
		WithGasBudget(10_000)
	tx, _, err = ch.PostRequestSyncTx(req, nil)
	require.NoError(t, err)
	reqs, err = ch.Env.RequestsForChain(tx, ch.ChainID)
	require.NoError(t, err)
	reqID = reqs[0].ID()
	checkNEvents(t, ch, reqID, 1)
}

func incrementSCCounter(t *testing.T, ch *solo.Chain) isc.RequestID {
	tx, _, err := ch.PostRequestSyncTx(
		solo.NewCallParams(incCounterName, incrementFn).WithGasBudget(math.MaxUint64),
		nil,
	)
	require.NoError(t, err)
	reqs, err := ch.Env.RequestsForChain(tx, ch.ChainID)
	require.NoError(t, err)
	return reqs[0].ID()
}

func TestGetEvents(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
	ch := env.NewChain()

	err := ch.DeployWasmContract(nil, incCounterName, "../../../../tools/cluster/tests/wasm/inccounter_bg.wasm")
	require.NoError(t, err)

	err = ch.DepositBaseTokensToL2(10_000, nil)
	require.NoError(t, err)

	reqID1 := incrementSCCounter(t, ch)
	reqID2 := incrementSCCounter(t, ch)
	reqID3 := incrementSCCounter(t, ch)

	events := ch.GetEventsForRequest(reqID1)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Contains(t, events[0], "counter = 1")
	events = ch.GetEventsForRequest(reqID2)
	require.Len(t, events, 1)
	require.Contains(t, events[0], "counter = 2")
	events = ch.GetEventsForRequest(reqID3)
	require.Len(t, events, 1)
	require.Contains(t, events[0], "counter = 3")

	events = ch.GetEventsForContract(incCounterName)
	require.Len(t, events, 3)
	require.Contains(t, events[0], "counter = 1")
	require.Contains(t, events[1], "counter = 2")
	require.Contains(t, events[2], "counter = 3")
}
