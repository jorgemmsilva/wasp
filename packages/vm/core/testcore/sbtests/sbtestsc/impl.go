package sbtestsc

import (
	"github.com/samber/lo"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
)

func initialize(ctx isc.Sandbox) dict.Dict {
	p := ctx.Params().Get(ParamFail)
	ctx.Requiref(p == nil, "failing on purpose")
	return nil
}

// testEventLogGenericData is called several times in log_test.go
func testEventLogGenericData(ctx isc.Sandbox) dict.Dict {
	params := ctx.Params()
	inc := lo.Must(codec.Uint64.Decode(params.Get(VarCounter), 1))
	eventCounter(ctx, inc)
	return nil
}

func testEventLogEventData(ctx isc.Sandbox) dict.Dict {
	eventTest(ctx)
	return nil
}

func testChainOwnerIDView(ctx isc.SandboxView) dict.Dict {
	cOwnerID := ctx.ChainOwnerID()
	return dict.Dict{ParamChainOwnerID: cOwnerID.Bytes()}
}

func testChainOwnerIDFull(ctx isc.Sandbox) dict.Dict {
	cOwnerID := ctx.ChainOwnerID()
	return dict.Dict{ParamChainOwnerID: cOwnerID.Bytes()}
}

func testSandboxCall(ctx isc.SandboxView) dict.Dict {
	ret := ctx.CallView(governance.ViewGetChainInfo.Message())
	return ret
}

func testEventLogDeploy(ctx isc.Sandbox) dict.Dict {
	// Deploy the same contract with another name
	ctx.DeployContract(Contract.ProgramHash, VarContractNameDeployed, nil)
	return nil
}

func testPanicFullEP(ctx isc.Sandbox) dict.Dict {
	ctx.Log().LogPanicf(MsgFullPanic)
	return nil
}

func testPanicViewEP(ctx isc.SandboxView) dict.Dict {
	ctx.Log().LogPanicf(MsgViewPanic)
	return nil
}

func testJustView(ctx isc.SandboxView) dict.Dict {
	ctx.Log().LogInfof("calling empty view entry point")
	return nil
}

func testCallPanicFullEP(ctx isc.Sandbox) dict.Dict {
	ctx.Log().LogInfof("will be calling entry point '%s' from full EP", FuncPanicFullEP)
	return ctx.Call(isc.NewMessage(Contract.Hname(), FuncPanicFullEP.Hname(), nil), nil)
}

func testCallPanicViewEPFromFull(ctx isc.Sandbox) dict.Dict {
	ctx.Log().LogInfof("will be calling entry point '%s' from full EP", FuncPanicViewEP)
	return ctx.Call(isc.NewMessage(Contract.Hname(), FuncPanicViewEP.Hname(), nil), nil)
}

func testCallPanicViewEPFromView(ctx isc.SandboxView) dict.Dict {
	ctx.Log().LogInfof("will be calling entry point '%s' from view EP", FuncPanicViewEP)
	return ctx.CallView(isc.NewMessage(Contract.Hname(), FuncPanicViewEP.Hname(), nil))
}

func doNothing(ctx isc.Sandbox) dict.Dict {
	ctx.Log().LogInfof(MsgDoNothing)
	return nil
}
