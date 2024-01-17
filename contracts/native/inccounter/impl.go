package inccounter

import (
	"fmt"
	"math"
	"time"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/kv/kvdecoder"
)

var Contract = coreutil.NewContract("inccounter")

var (
	FuncIncCounter = coreutil.NewEP1(Contract, "incCounter",
		coreutil.FieldWithCodecOptional(VarCounter, codec.Int64),
	)
	FuncIncAndRepeatOnceAfter2s = coreutil.NewEP0(Contract, "incAndRepeatOnceAfter5s")
	FuncIncAndRepeatMany        = coreutil.NewEP2(Contract, "incAndRepeatMany",
		coreutil.FieldWithCodecOptional(VarCounter, codec.Int64),
		coreutil.FieldWithCodecOptional(VarNumRepeats, codec.Int64),
	)
	FuncSpawn = coreutil.NewEP1(Contract, "spawn",
		coreutil.FieldWithCodec(VarName, codec.String),
	)
	ViewGetCounter = coreutil.NewViewEP01(Contract, "getCounter",
		coreutil.FieldWithCodec(VarCounter, codec.Int64),
	)
)

var Processor = Contract.Processor(initialize,
	FuncIncCounter.WithHandler(incCounter),
	FuncIncAndRepeatOnceAfter2s.WithHandler(incCounterAndRepeatOnce),
	FuncIncAndRepeatMany.WithHandler(incCounterAndRepeatMany),
	FuncSpawn.WithHandler(spawn),
	ViewGetCounter.WithHandler(getCounter),
)

func InitParams(initialValue int64) dict.Dict {
	return dict.Dict{VarCounter: codec.Int64.Encode(initialValue)}
}

const (
	VarNumRepeats = "numRepeats"
	VarCounter    = "counter"
	VarName       = "name"
)

func initialize(ctx isc.Sandbox) dict.Dict {
	ctx.Log().LogDebugf("inccounter.init in %s", ctx.Contract().String())
	params := ctx.Params()
	val := lo.Must(codec.Int64.Decode(params.Get(VarCounter), 0))
	ctx.State().Set(VarCounter, codec.Int64.Encode(val))
	eventCounter(ctx, val)
	return nil
}

func incCounter(ctx isc.Sandbox, incOpt *int64) dict.Dict {
	inc := coreutil.FromOptional(incOpt, 1)
	ctx.Log().LogDebugf("inccounter.incCounter in %s", ctx.Contract().String())
	state := kvdecoder.New(ctx.State(), ctx.Log())
	val := state.MustGetInt64(VarCounter, 0)
	ctx.Log().LogInfof("incCounter: increasing counter value %d by %d, anchor index: #%d",
		val, inc, ctx.StateAnchor().StateIndex)
	tra := "(empty)"
	if ctx.AllowanceAvailable() != nil {
		tra = ctx.AllowanceAvailable().String()
	}
	ctx.Log().LogInfof("incCounter: allowance available: %s", tra)
	ctx.State().Set(VarCounter, codec.Int64.Encode(val+inc))
	eventCounter(ctx, val+inc)
	return nil
}

func incCounterAndRepeatOnce(ctx isc.Sandbox) dict.Dict {
	ctx.Log().LogDebugf("inccounter.incCounterAndRepeatOnce")
	state := ctx.State()
	val := lo.Must(codec.Int64.Decode(state.Get(VarCounter), 0))

	ctx.Log().LogDebugf(fmt.Sprintf("incCounterAndRepeatOnce: increasing counter value: %d", val))
	state.Set(VarCounter, codec.Int64.Encode(val+1))
	eventCounter(ctx, val+1)
	allowance := ctx.AllowanceAvailable()
	ctx.TransferAllowedFunds(ctx.AccountID())
	ctx.Send(isc.RequestParameters{
		TargetAddress:                 ctx.ChainID().AsAddress(),
		Assets:                        isc.NewAssets(allowance.BaseTokens, nil),
		AdjustToMinimumStorageDeposit: true,
		Metadata: &isc.SendMetadata{
			Message:   isc.NewMessage(ctx.Contract(), FuncIncCounter.Hname()),
			GasBudget: math.MaxUint64,
		},
		UnlockConditions: []iotago.UnlockCondition{
			&iotago.TimelockUnlockCondition{
				Slot: ctx.L1API().TimeProvider().SlotFromTime(ctx.Timestamp().Add(200 * time.Second)),
			},
		},
	})
	ctx.Log().LogDebugf("incCounterAndRepeatOnce: PostRequestToSelfWithDelay RequestInc 2 sec")
	return nil
}

func incCounterAndRepeatMany(ctx isc.Sandbox, valOpt, numRepeatsOpt *int64) dict.Dict {
	val := coreutil.FromOptional(valOpt, 0)
	numRepeats := coreutil.FromOptional(valOpt, lo.Must(codec.Int64.Decode(ctx.State().Get(VarNumRepeats), 0)))
	ctx.Log().LogDebugf("inccounter.incCounterAndRepeatMany")

	state := ctx.State()

	state.Set(VarCounter, codec.Int64.Encode(val+1))
	eventCounter(ctx, val+1)
	ctx.Log().LogDebugf("inccounter.incCounterAndRepeatMany: increasing counter value: %d", val)

	if numRepeats == 0 {
		ctx.Log().LogDebugf("inccounter.incCounterAndRepeatMany: finished chain of requests. counter value: %d", val)
		return nil
	}

	ctx.Log().LogDebugf("chain of %d requests ahead", numRepeats)

	state.Set(VarNumRepeats, codec.Int64.Encode(numRepeats-1))
	ctx.TransferAllowedFunds(ctx.AccountID())
	ctx.Send(isc.RequestParameters{
		TargetAddress:                 ctx.ChainID().AsAddress(),
		Assets:                        isc.NewAssets(1000, nil),
		AdjustToMinimumStorageDeposit: true,
		Metadata: &isc.SendMetadata{
			Message:   isc.NewMessage(ctx.Contract(), FuncIncAndRepeatMany.Hname()),
			GasBudget: math.MaxUint64,
			Allowance: isc.NewAssetsBaseTokens(1000),
		},
		UnlockConditions: []iotago.UnlockCondition{
			&iotago.TimelockUnlockCondition{
				Slot: ctx.L1API().TimeProvider().SlotFromTime(ctx.Timestamp().Add(200 * time.Second)),
			},
		},
	})

	ctx.Log().LogDebugf("incCounterAndRepeatMany. remaining repeats = %d", numRepeats-1)
	return nil
}

// spawn deploys new contract and calls it
func spawn(ctx isc.Sandbox, name string) dict.Dict {
	ctx.Log().LogDebugf("inccounter.spawn")

	state := kvdecoder.New(ctx.State(), ctx.Log())
	val := state.MustGetInt64(VarCounter)

	callPar := dict.New()
	callPar.Set(VarCounter, codec.Int64.Encode(val+1))
	eventCounter(ctx, val+1)
	ctx.DeployContract(Contract.ProgramHash, name, callPar)

	// increase counter in newly spawned contract
	ctx.Call(FuncIncCounter.Message(nil), nil)

	return nil
}

func getCounter(ctx isc.SandboxView) int64 {
	return lo.Must(codec.Int64.Decode(ctx.StateR().Get(VarCounter), 0))
}
