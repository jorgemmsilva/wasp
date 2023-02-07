package sbtestsc

import (
	"strings"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/kv/kvdecoder"
	"github.com/iotaledger/wasp/packages/util"
)

// ParamCallOption
// ParamCallIntParam
// ParamHnameContract
func callOnChain(ctx isc.Sandbox) []byte {
	ctx.Log().Debugf(FuncCallOnChain.Name)
	params := kvdecoder.New(ctx.Params(), ctx.Log())
	paramIn := params.MustGetUint64(ParamN)
	hnameContract := params.MustGetHname(ParamHnameContract, ctx.Contract())
	hnameEP := params.MustGetHname(ParamHnameEP, FuncCallOnChain.Hname())

	state := kvdecoder.New(ctx.State(), ctx.Log())
	counter := state.MustGetUint64(VarCounter, 0)
	ctx.State().Set(VarCounter, codec.EncodeUint64(counter+1))

	ctx.Log().Infof("param IN = %d, hnameContract = %s, hnameEP = %s, counter = %d",
		paramIn, hnameContract, hnameEP, counter)

	return ctx.Call(hnameContract, hnameEP, codec.MakeDict(map[string]interface{}{
		ParamN: paramIn,
	}), nil)
}

func incCounter(ctx isc.Sandbox) []byte {
	state := kvdecoder.New(ctx.State(), ctx.Log())
	counter := state.MustGetUint64(VarCounter, 0)
	ctx.State().Set(VarCounter, codec.EncodeUint64(counter+1))
	return nil
}

func getCounter(ctx isc.SandboxView) []byte {
	state := kvdecoder.New(ctx.StateR(), ctx.Log())
	counter := state.MustGetUint64(VarCounter, 0)
	return util.MustSerialize(counter)
}

func runRecursion(ctx isc.Sandbox) []byte {
	params := kvdecoder.New(ctx.Params(), ctx.Log())
	depth := params.MustGetUint64(ParamN)
	if depth == 0 {
		return nil
	}
	return ctx.Call(ctx.Contract(), FuncCallOnChain.Hname(), codec.MakeDict(map[string]interface{}{
		ParamHnameEP: FuncRunRecursion.Hname(),
		ParamN:       depth - 1,
	}), nil)
}

func fibonacci(n uint64) uint64 {
	if n <= 1 {
		return n
	}
	return fibonacci(n-1) + fibonacci(n-2)
}

func getFibonacci(ctx isc.SandboxView) []byte {
	params := kvdecoder.New(ctx.Params(), ctx.Log())
	n := params.MustGetUint64(ParamN)
	ctx.Log().Infof("fibonacci( %d )", n)
	return util.MustSerialize(fibonacci(n))
}

func getFibonacciIndirect(ctx isc.SandboxView) []byte {
	params := kvdecoder.New(ctx.Params(), ctx.Log())

	n := params.MustGetUint64(ParamN)
	ctx.Log().Infof("fibonacciIndirect( %d )", n)
	if n <= 1 {
		return util.MustSerialize(n)
	}

	ret1 := ctx.CallView(ctx.Contract(), FuncGetFibonacciIndirect.Hname(), codec.MakeDict(map[string]interface{}{
		ParamN: n - 1,
	}))

	n1 := util.MustDeserialize[uint64](ret1)

	ret2 := ctx.CallView(ctx.Contract(), FuncGetFibonacciIndirect.Hname(), codec.MakeDict(map[string]interface{}{
		ParamN: n - 2,
	}))
	n2 := util.MustDeserialize[uint64](ret2)

	return util.MustSerialize(n1 + n2)
}

// calls the "fib indirect" view and stores the result in the state
func calcFibonacciIndirectStoreValue(ctx isc.Sandbox) []byte {
	ret := ctx.CallView(ctx.Contract(), FuncGetFibonacciIndirect.Hname(), dict.Dict{
		ParamN: ctx.Params().MustGet(ParamN),
	})
	ctx.State().Set(ParamN, codec.Encode(util.MustDeserialize[uint64](ret)))
	return nil
}

func viewFibResult(ctx isc.SandboxView) []byte {
	return ctx.StateR().MustGet(ParamN)
}

// ParamIntParamName
// ParamIntParamValue
func setInt(ctx isc.Sandbox) []byte {
	ctx.Log().Infof(FuncSetInt.Name)
	params := kvdecoder.New(ctx.Params(), ctx.Log())
	paramName := params.MustGetString(ParamIntParamName)
	paramValue := params.MustGetInt64(ParamIntParamValue)
	ctx.State().Set(kv.Key(paramName), codec.EncodeInt64(paramValue))
	return nil
}

// ParamIntParamName
func getInt(ctx isc.SandboxView) []byte {
	ctx.Log().Infof(FuncGetInt.Name)
	params := kvdecoder.New(ctx.Params(), ctx.Log())
	paramName := params.MustGetString(ParamIntParamName)
	state := kvdecoder.New(ctx.StateR(), ctx.Log())
	return util.MustSerialize(state.MustGetInt64(kv.Key(paramName), 0))
}

func infiniteLoop(ctx isc.Sandbox) []byte {
	for {
		// do nothing, just waste gas
		ctx.State().Set("foo", []byte(strings.Repeat("dummy data", 1000)))
	}
}

func infiniteLoopView(ctx isc.SandboxView) []byte {
	for {
		// do nothing, just waste gas
		ctx.CallView(ctx.Contract(), FuncGetCounter.Hname(), dict.Dict{})
	}
}
