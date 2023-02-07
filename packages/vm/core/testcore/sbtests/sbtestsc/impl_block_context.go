package sbtestsc

import (
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
)

type blockCtx struct {
	numCalls uint8
}

func openBlockContext(ctx isc.Sandbox) []byte {
	ctx.RequireCallerIsChainOwner()
	ctx.Privileged().SetBlockContext(&blockCtx{})
	return nil
}

func closeBlockContext(ctx isc.Sandbox) []byte {
	ctx.RequireCallerIsChainOwner()
	ctx.State().Set("numCalls", codec.EncodeUint8(getBlockContext(ctx).numCalls))
	return nil
}

func getBlockContext(ctx isc.Sandbox) *blockCtx {
	return ctx.Privileged().BlockContext().(*blockCtx)
}

func getLastBlockNumCalls(ctx isc.SandboxView) []byte {
	return ctx.StateR().MustGet("numCalls")
}
