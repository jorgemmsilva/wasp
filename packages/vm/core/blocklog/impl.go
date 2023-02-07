package blocklog

import (
	"math"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/util"
)

var Processor = Contract.Processor(initialize,
	ViewControlAddresses.WithHandler(viewControlAddresses),
	ViewGetBlockInfo.WithHandler(viewGetBlockInfo),
	ViewGetEventsForBlock.WithHandler(viewGetEventsForBlock),
	ViewGetEventsForContract.WithHandler(viewGetEventsForContract),
	ViewGetEventsForRequest.WithHandler(viewGetEventsForRequest),
	ViewGetRequestIDsForBlock.WithHandler(viewGetRequestIDsForBlock),
	ViewGetRequestReceipt.WithHandler(viewGetRequestReceipt),
	ViewGetRequestReceiptsForBlock.WithHandler(viewGetRequestReceiptsForBlock),
	ViewIsRequestProcessed.WithHandler(viewIsRequestProcessed),
)

func initialize(ctx isc.Sandbox) []byte {
	SaveNextBlockInfo(ctx.State(), &BlockInfo{
		BlockIndex:            0,
		Timestamp:             ctx.Timestamp(),
		TotalRequests:         1,
		NumSuccessfulRequests: 1,
		NumOffLedgerRequests:  0,
		PreviousL1Commitment:  *state.OriginL1Commitment(),
		L1Commitment:          nil, // not known yet
	})
	// storing hname as a terminal value of the contract's state root.
	// This way we will be able to retrieve commitment to the contract's state
	ctx.State().Set("", ctx.Contract().Bytes())

	ctx.Log().Debugf("blocklog.initialize.success hname = %s", Contract.Hname().String())
	return nil
}

type ControllingAddressReturn struct {
	StateAddress     []byte
	GoverningAddress []byte
	BlockIndex       uint32
}

func viewControlAddresses(ctx isc.SandboxView) []byte {
	registry := collections.NewArray32ReadOnly(ctx.StateR(), prefixControlAddresses)
	l := registry.MustLen()
	ctx.Requiref(l > 0, "inconsistency: unknown control addresses")
	rec, err := ControlAddressesFromBytes(registry.MustGetAt(l - 1))
	ctx.RequireNoError(err)
	return util.MustSerialize(ControllingAddressReturn{
		StateAddress:     isc.BytesFromAddress(rec.StateAddress),
		GoverningAddress: isc.BytesFromAddress(rec.GoverningAddress),
		BlockIndex:       rec.SinceBlockIndex,
	})
}

// viewGetBlockInfo returns blockInfo for a given block.
// params:
// ParamBlockIndex - index of the block (defaults to the latest block)
func viewGetBlockInfo(ctx isc.SandboxView) []byte {
	blockIndex := getBlockIndexParams(ctx)
	data, err := getBlockInfoBytes(ctx.StateR(), blockIndex)
	ctx.RequireNoError(err)
	// TODO return block index?...........
	return data
}

// viewGetRequestIDsForBlock returns a list of requestIDs for a given block.
// params:
// ParamBlockIndex - index of the block (defaults to latest block)
func viewGetRequestIDsForBlock(ctx isc.SandboxView) []byte {
	blockIndex := getBlockIndexParams(ctx)

	if blockIndex == 0 {
		// block 0 is an empty state
		return nil
	}

	dataArr, found, err := getRequestLogRecordsForBlockBin(ctx.StateR(), blockIndex)
	ctx.RequireNoError(err)
	ctx.Requiref(found, "not found")

	ret := dict.New()
	arr := collections.NewArray16(ret, ParamRequestID)
	for _, d := range dataArr {
		rec, err := RequestReceiptFromBytes(d)
		ctx.RequireNoError(err)
		arr.MustPush(rec.Request.ID().Bytes())
	}
	return ret.Bytes()
}

func viewGetRequestReceipt(ctx isc.SandboxView) []byte {
	requestID := ctx.Params().MustGetRequestID(ParamRequestID)
	res, err := GetRequestRecordDataByRequestID(ctx.StateR(), requestID)
	ctx.RequireNoError(err)
	if res == nil {
		return nil
	}
	return dict.Dict{
		ParamRequestRecord: res.ReceiptBin,
		ParamBlockIndex:    codec.EncodeUint32(res.BlockIndex),
		ParamRequestIndex:  codec.EncodeUint16(res.RequestIndex),
	}.Bytes()
}

// viewGetRequestReceiptsForBlock returns a list of receipts for a given block.
// params:
// ParamBlockIndex - index of the block (defaults to latest block)
func viewGetRequestReceiptsForBlock(ctx isc.SandboxView) []byte {
	blockIndex := getBlockIndexParams(ctx)

	if blockIndex == 0 {
		// block 0 is an empty state
		return nil
	}

	dataArr, found, err := getRequestLogRecordsForBlockBin(ctx.StateR(), blockIndex)
	ctx.RequireNoError(err)
	ctx.Requiref(found, "not found")
	return util.MustSerialize(dataArr)
}

func viewIsRequestProcessed(ctx isc.SandboxView) []byte {
	requestID := ctx.Params().MustGetRequestID(ParamRequestID)
	requestReceipt, err := isRequestProcessedInternal(ctx.StateR(), requestID)
	ctx.RequireNoError(err)
	return util.MustSerialize(requestReceipt != nil)
}

// viewGetEventsForRequest returns a list of events for a given request.
// params:
// ParamRequestID - requestID
func viewGetEventsForRequest(ctx isc.SandboxView) []byte {
	requestID := ctx.Params().MustGetRequestID(ParamRequestID)

	events, err := getRequestEventsInternal(ctx.StateR(), requestID)
	ctx.RequireNoError(err)
	return util.MustSerialize(events)
}

// viewGetEventsForBlock returns a list of events for a given block.
// params:
// ParamBlockIndex - index of the block (defaults to latest block)
func viewGetEventsForBlock(ctx isc.SandboxView) []byte {
	blockIndex := getBlockIndexParams(ctx)

	if blockIndex == 0 {
		// block 0 is an empty state
		return nil
	}

	blockInfo, err := GetBlockInfo(ctx.StateR(), blockIndex)
	ctx.RequireNoError(err)
	events, err := GetEventsByBlockIndex(ctx.StateR(), blockIndex, blockInfo.TotalRequests)
	ctx.RequireNoError(err)

	return util.MustSerialize(events)
}

// viewGetEventsForContract returns a list of events for a given smart contract.
// params:
// ParamContractHname - hname of the contract
// ParamFromBlock - defaults to 0
// ParamToBlock - defaults to latest block
func viewGetEventsForContract(ctx isc.SandboxView) []byte {
	contract := ctx.Params().MustGetHname(ParamContractHname)
	fromBlock := ctx.Params().MustGetUint32(ParamFromBlock, 0)
	toBlock := ctx.Params().MustGetUint32(ParamToBlock, math.MaxUint32)
	events, err := getSmartContractEventsInternal(ctx.StateR(), contract, fromBlock, toBlock)
	ctx.RequireNoError(err)

	return util.MustSerialize(events)
}
