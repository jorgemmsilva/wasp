// in the blocklog core contract the VM keeps indices of blocks and requests in an optimized way
// for fast checking and timestamp access.
package blocklog

import (
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/kv/dict"
)

var Contract = coreutil.NewContract(coreutil.CoreContractBlocklog)

var (
	// Funcs
	FuncRetryUnprocessable = coreutil.NewEP1(Contract, "retryUnprocessable",
		ParamRequestID, codec.RequestID,
	)

	// Views
	ViewGetBlockInfo = coreutil.NewViewEP12(Contract, "getBlockInfo",
		ParamBlockIndex, codec.Uint32, // optional
		ParamBlockIndex, codec.Uint32,
		ParamBlockInfo, codec.NewCodecEx(BlockInfoFromBytes),
	)
	ViewGetRequestIDsForBlock = EPViewGetRequestIDsForBlock{
		EP1: coreutil.NewViewEP1(Contract, "getRequestIDsForBlock",
			ParamBlockIndex, codec.Uint32, // optional
		),
	}
	ViewGetRequestReceipt = EPViewGetRequestReceipt{
		EP1: coreutil.NewViewEP1(Contract, "getRequestReceipt",
			ParamRequestID, codec.RequestID,
		),
	}
	ViewGetRequestReceiptsForBlock = EPViewGetRequestReceiptsForBlock{
		EP1: coreutil.NewViewEP1(Contract, "getRequestReceiptsForBlock",
			ParamBlockIndex, codec.Uint32, // optional
		),
	}
	ViewIsRequestProcessed = coreutil.NewViewEP11(Contract, "isRequestProcessed",
		ParamRequestID, codec.RequestID,
		ParamRequestProcessed, codec.Bool,
	)
	ViewGetEventsForRequest = EPViewGetEventsForRequest{
		EP1: coreutil.NewViewEP1(Contract, "getEventsForRequest",
			ParamRequestID, codec.RequestID,
		),
	}
	ViewGetEventsForBlock = EPViewGetEventsForBlock{
		EP1: coreutil.NewViewEP1(Contract, "getEventsForBlock",
			ParamBlockIndex, codec.Uint32, // optional
		),
	}
	ViewGetEventsForContract = EPViewGetEventsForContract{
		EP1: coreutil.NewViewEP1(Contract, "getEventsForContract",
			ParamContractHname, codec.Hname,
		),
	}
	ViewHasUnprocessable = coreutil.NewViewEP11(Contract, "hasUnprocessable",
		ParamRequestID, codec.RequestID,
		ParamUnprocessableRequestExists, codec.Bool,
	)
)

// request parameters
const (
	ParamBlockIndex                 = "n"
	ParamBlockInfo                  = "i"
	ParamContractHname              = "h"
	ParamFromBlock                  = "f"
	ParamToBlock                    = "t"
	ParamRequestID                  = "u"
	ParamRequestIndex               = "r"
	ParamRequestProcessed           = "p"
	ParamRequestRecord              = "d"
	ParamEvent                      = "e"
	ParamStateControllerAddress     = "s"
	ParamUnprocessableRequestExists = "x"
)

const (
	// Array of blockIndex => BlockInfo (pruned)
	PrefixBlockRegistry = "a"

	// Map of request.ID().LookupDigest() => []RequestLookupKey (pruned)
	//   LookupDigest = reqID[:6] | outputIndex
	//   RequestLookupKey = blockIndex | requestIndex
	prefixRequestLookupIndex = "b"

	// Map of RequestLookupKey => RequestReceipt (pruned)
	//   RequestLookupKey = blockIndex | requestIndex
	prefixRequestReceipts = "c"

	// Map of EventLookupKey => event (pruned)
	//   EventLookupKey = blockIndex | requestIndex | eventIndex
	prefixRequestEvents = "d"

	// Map of requestID => unprocessableRequestRecord
	prefixUnprocessableRequests = "u"

	// Array of requestID.
	// Temporary list of unprocessable requests that need updating the outputID field
	prefixNewUnprocessableRequests = "U"
)

type EPViewGetRequestIDsForBlock struct {
	coreutil.EP1[isc.SandboxView, uint32]
	Output EPViewRequestIDsOutput
}

type EPViewRequestIDsOutput struct{}

func (e EPViewRequestIDsOutput) Decode(r dict.Dict) ([]isc.RequestID, error) {
	requestIDs := collections.NewArrayReadOnly(r, ParamRequestID)
	ret := make([]isc.RequestID, requestIDs.Len())
	for i := range ret {
		var err error
		ret[i], err = isc.RequestIDFromBytes(requestIDs.GetAt(uint32(i)))
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

type EPViewGetRequestReceipt struct {
	coreutil.EP1[isc.SandboxView, isc.RequestID]
	Output RequestReceiptOutput
}

type RequestReceiptOutput struct{}

func (e RequestReceiptOutput) Decode(r dict.Dict) (*RequestReceipt, bool, error) {
	if r.IsEmpty() {
		return nil, false, nil
	}
	blockIndex, err := codec.Uint32.Decode(r[ParamBlockIndex])
	if err != nil {
		return nil, false, err
	}
	reqIndex, err := codec.Uint16.Decode(r[ParamRequestIndex])
	if err != nil {
		return nil, false, err
	}
	rec, err := RequestReceiptFromBytes(r[ParamRequestRecord], blockIndex, reqIndex)
	if err != nil {
		return nil, false, err
	}
	return rec, true, nil
}

type EPViewGetRequestReceiptsForBlock struct {
	coreutil.EP1[isc.SandboxView, uint32]
	Output RequestReceiptsOutput
}

type RequestReceiptsOutput struct{}

func (e RequestReceiptsOutput) Decode(r dict.Dict) ([]*RequestReceipt, error) {
	receipts := collections.NewArrayReadOnly(r, ParamRequestRecord)
	ret := make([]*RequestReceipt, receipts.Len())
	var err error
	blockIndex, err := codec.Uint32.Decode(r.Get(ParamBlockIndex))
	if err != nil {
		return nil, err
	}
	for i := range ret {
		ret[i], err = RequestReceiptFromBytes(receipts.GetAt(uint32(i)), blockIndex, uint16(i))
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

type EventsOutput struct{}

func (e EventsOutput) Decode(r dict.Dict) ([]*isc.Event, error) {
	events := collections.NewArrayReadOnly(r, ParamEvent)
	ret := make([]*isc.Event, events.Len())
	for i := range ret {
		var err error
		ret[i], err = isc.EventFromBytes(events.GetAt(uint32(i)))
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

type EPViewGetEventsForRequest struct {
	coreutil.EP1[isc.SandboxView, isc.RequestID]
	Output EventsOutput
}

type EPViewGetEventsForBlock struct {
	coreutil.EP1[isc.SandboxView, uint32]
	Output EventsOutput
}

type EPViewGetEventsForContract struct {
	coreutil.EP1[isc.SandboxView, isc.Hname]
	Output EventsOutput
}

func (e EPViewGetEventsForContract) MessageWithRange(contract isc.Hname, fromBlock, toBlock uint32) isc.Message {
	msg := e.Message(contract)
	msg.Params[ParamFromBlock] = codec.Uint32.Encode(fromBlock)
	msg.Params[ParamToBlock] = codec.Uint32.Encode(toBlock)
	return msg
}
