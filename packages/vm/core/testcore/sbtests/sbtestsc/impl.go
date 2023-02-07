package sbtestsc

import (
	"fmt"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
)

func initialize(ctx isc.Sandbox) []byte {
	p := ctx.Params().MustGet(ParamFail)
	ctx.Requiref(p == nil, "failing on purpose")
	return nil
}

// testEventLogGenericData is called several times in log_test.go
func testEventLogGenericData(ctx isc.Sandbox) []byte {
	params := ctx.Params()
	inc := codec.MustDecodeUint64(params.MustGet(VarCounter), 1)
	ctx.Event(fmt.Sprintf("[GenericData] Counter Number: %d", inc))
	return nil
}

func testEventLogEventData(ctx isc.Sandbox) []byte {
	ctx.Event("[Event] - Testing Event...")
	return nil
}

func testChainOwnerIDView(ctx isc.SandboxView) []byte {
	cOwnerID := ctx.ChainOwnerID()
	return cOwnerID.Bytes()
}

func testChainOwnerIDFull(ctx isc.Sandbox) []byte {
	cOwnerID := ctx.ChainOwnerID()
	return cOwnerID.Bytes()
}

func testSandboxCall(ctx isc.SandboxView) []byte {
	ret := ctx.CallView(governance.Contract.Hname(), governance.ViewGetChainInfo.Hname(), nil)
	chInfo := util.MustDeserialize[governance.ChainInfo](ret)
	return util.MustSerialize(chInfo.Description)
}

func testEventLogDeploy(ctx isc.Sandbox) []byte {
	// Deploy the same contract with another name
	ctx.DeployContract(Contract.ProgramHash,
		VarContractNameDeployed, "test contract deploy log", nil)
	return nil
}

func testPanicFullEP(ctx isc.Sandbox) []byte {
	ctx.Log().Panicf(MsgFullPanic)
	return nil
}

func testPanicViewEP(ctx isc.SandboxView) []byte {
	ctx.Log().Panicf(MsgViewPanic)
	return nil
}

func testJustView(ctx isc.SandboxView) []byte {
	ctx.Log().Infof("calling empty view entry point")
	return nil
}

func testCallPanicFullEP(ctx isc.Sandbox) []byte {
	ctx.Log().Infof("will be calling entry point '%s' from full EP", FuncPanicFullEP)
	return ctx.Call(Contract.Hname(), FuncPanicFullEP.Hname(), nil, nil)
}

func testCallPanicViewEPFromFull(ctx isc.Sandbox) []byte {
	ctx.Log().Infof("will be calling entry point '%s' from full EP", FuncPanicViewEP)
	return ctx.Call(Contract.Hname(), FuncPanicViewEP.Hname(), nil, nil)
}

func testCallPanicViewEPFromView(ctx isc.SandboxView) []byte {
	ctx.Log().Infof("will be calling entry point '%s' from view EP", FuncPanicViewEP)
	return ctx.CallView(Contract.Hname(), FuncPanicViewEP.Hname(), nil)
}

func doNothing(ctx isc.Sandbox) []byte {
	ctx.Log().Infof(MsgDoNothing)
	return nil
}
