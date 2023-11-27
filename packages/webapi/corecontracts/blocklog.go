package corecontracts

import (
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/kv/kvdecoder"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
)

func GetControlAddresses(ch chaintypes.Chain) (*isc.ControlAddresses, error) {
	chainOutput, err := ch.LatestChainOutputs(chaintypes.ConfirmedState)
	if err != nil {
		return nil, err
	}
	anchorOutput := chainOutput.AnchorOutput

	controlAddresses := &isc.ControlAddresses{
		StateAddress:     chainOutput.AnchorOutput.StateController(),
		GoverningAddress: anchorOutput.GovernorAddress(),
		SinceBlockIndex:  anchorOutput.StateIndex,
	}

	return controlAddresses, nil
}

func handleBlockInfo(info dict.Dict) (*blocklog.BlockInfo, error) {
	resultDecoder := kvdecoder.New(info)

	blockInfoBin, err := resultDecoder.GetBytes(blocklog.ParamBlockInfo)
	if err != nil {
		return nil, err
	}

	blockInfo, err := blocklog.BlockInfoFromBytes(blockInfoBin)
	if err != nil {
		return nil, err
	}

	return blockInfo, nil
}

func GetLatestBlockInfo(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) (*blocklog.BlockInfo, error) {
	_, ret, err := callViewInvoker(blocklog.Contract.Hname(), blocklog.ViewGetBlockInfo.Hname(), nil, blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}

	return handleBlockInfo(ret)
}

func GetBlockInfo(callViewInvoker CallViewInvoker, blockIndex uint32, blockIndexOrTrieRoot string) (*blocklog.BlockInfo, error) {
	_, ret, err := callViewInvoker(blocklog.Contract.Hname(), blocklog.ViewGetBlockInfo.Hname(), codec.MakeDict(map[string]interface{}{
		blocklog.ParamBlockIndex: blockIndex,
	}), blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}

	return handleBlockInfo(ret)
}

func handleRequestIDs(requestIDsDict dict.Dict) (ret []isc.RequestID, err error) {
	requestIDs := collections.NewArrayReadOnly(requestIDsDict, blocklog.ParamRequestID)
	ret = make([]isc.RequestID, requestIDs.Len())
	for i := range ret {
		ret[i], err = isc.RequestIDFromBytes(requestIDs.GetAt(uint32(i)))
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func GetRequestIDsForLatestBlock(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) ([]isc.RequestID, error) {
	_, ret, err := callViewInvoker(blocklog.Contract.Hname(), blocklog.ViewGetRequestIDsForBlock.Hname(), nil, blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}

	return handleRequestIDs(ret)
}

func GetRequestIDsForBlock(callViewInvoker CallViewInvoker, blockIndex uint32, blockIndexOrTrieRoot string) ([]isc.RequestID, error) {
	_, ret, err := callViewInvoker(
		blocklog.Contract.Hname(),
		blocklog.ViewGetRequestIDsForBlock.Hname(),
		codec.MakeDict(map[string]interface{}{
			blocklog.ParamBlockIndex: blockIndex,
		}),
		blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}

	return handleRequestIDs(ret)
}

func GetRequestReceipt(callViewInvoker CallViewInvoker, requestID isc.RequestID, blockIndexOrTrieRoot string) (*blocklog.RequestReceipt, error) {
	_, ret, err := callViewInvoker(
		blocklog.Contract.Hname(),
		blocklog.ViewGetRequestReceipt.Hname(),
		codec.MakeDict(map[string]interface{}{
			blocklog.ParamRequestID: requestID,
		}),
		blockIndexOrTrieRoot,
	)
	if err != nil || ret == nil {
		return nil, err
	}

	resultDecoder := kvdecoder.New(ret)

	binRec, err := resultDecoder.GetBytes(blocklog.ParamRequestRecord)
	if err != nil {
		return nil, err
	}
	blockIndex, err := resultDecoder.GetUint32(blocklog.ParamBlockIndex)
	if err != nil {
		return nil, err
	}
	requestIndex, err := resultDecoder.GetUint16(blocklog.ParamRequestIndex)
	if err != nil {
		return nil, err
	}
	return blocklog.RequestReceiptFromBytes(binRec, blockIndex, requestIndex)
}

func GetRequestReceiptsForBlock(callViewInvoker CallViewInvoker, blockIndex uint32, blockIndexOrTrieRoot string) ([]*blocklog.RequestReceipt, error) {
	_, res, err := callViewInvoker(
		blocklog.Contract.Hname(),
		blocklog.ViewGetRequestReceiptsForBlock.Hname(),
		codec.MakeDict(map[string]interface{}{blocklog.ParamBlockIndex: blockIndex}),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return nil, err
	}
	return blocklog.ReceiptsFromViewCallResult(res)
}

func IsRequestProcessed(callViewInvoker CallViewInvoker, requestID isc.RequestID, blockIndexOrTrieRoot string) (bool, error) {
	_, ret, err := callViewInvoker(
		blocklog.Contract.Hname(),
		blocklog.ViewIsRequestProcessed.Hname(),
		codec.MakeDict(map[string]interface{}{blocklog.ParamRequestID: requestID}),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return false, err
	}

	resultDecoder := kvdecoder.New(ret)
	isProcessed, err := resultDecoder.GetBool(blocklog.ParamRequestProcessed)
	if err != nil {
		return false, err
	}

	return isProcessed, nil
}

func GetEventsForRequest(callViewInvoker CallViewInvoker, requestID isc.RequestID, blockIndexOrTrieRoot string) ([]*isc.Event, error) {
	_, ret, err := callViewInvoker(
		blocklog.Contract.Hname(),
		blocklog.ViewGetEventsForRequest.Hname(),
		codec.MakeDict(map[string]interface{}{blocklog.ParamRequestID: requestID}),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return nil, err
	}

	return blocklog.EventsFromViewResult(ret)
}

func GetEventsForBlock(callViewInvoker CallViewInvoker, blockIndex uint32, blockIndexOrTrieRoot string) ([]*isc.Event, error) {
	_, ret, err := callViewInvoker(
		blocklog.Contract.Hname(),
		blocklog.ViewGetEventsForBlock.Hname(),
		codec.MakeDict(map[string]interface{}{
			blocklog.ParamBlockIndex: blockIndex,
		}),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return nil, err
	}

	return blocklog.EventsFromViewResult(ret)
}

func GetEventsForContract(callViewInvoker CallViewInvoker, contractHname isc.Hname, blockIndexOrTrieRoot string) ([]*isc.Event, error) {
	_, ret, err := callViewInvoker(
		blocklog.Contract.Hname(),
		blocklog.ViewGetEventsForContract.Hname(),
		codec.MakeDict(map[string]interface{}{
			blocklog.ParamContractHname: contractHname,
		}),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return nil, err
	}

	return blocklog.EventsFromViewResult(ret)
}
