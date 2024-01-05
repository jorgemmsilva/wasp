package corecontracts

import (
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/isc"
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

func GetLatestBlockInfo(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) (*blocklog.BlockInfo, error) {
	_, ret, err := callViewInvoker(blocklog.ViewGetBlockInfo.Message(nil), blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}
	return blocklog.ViewGetBlockInfo.Output2.Decode(ret)
}

func GetBlockInfo(callViewInvoker CallViewInvoker, blockIndex uint32, blockIndexOrTrieRoot string) (*blocklog.BlockInfo, error) {
	_, ret, err := callViewInvoker(blocklog.ViewGetBlockInfo.Message(&blockIndex), blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}
	return blocklog.ViewGetBlockInfo.Output2.Decode(ret)
}

func GetRequestIDsForLatestBlock(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) ([]isc.RequestID, error) {
	_, ret, err := callViewInvoker(blocklog.ViewGetRequestIDsForBlock.Message(nil), blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}
	return blocklog.ViewGetRequestIDsForBlock.Output2.Decode(ret)
}

func GetRequestIDsForBlock(callViewInvoker CallViewInvoker, blockIndex uint32, blockIndexOrTrieRoot string) ([]isc.RequestID, error) {
	_, ret, err := callViewInvoker(blocklog.ViewGetRequestIDsForBlock.Message(&blockIndex), blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}
	return blocklog.ViewGetRequestIDsForBlock.Output2.Decode(ret)
}

func GetRequestReceipt(callViewInvoker CallViewInvoker, requestID isc.RequestID, blockIndexOrTrieRoot string) (*blocklog.RequestReceipt, bool, error) {
	_, ret, err := callViewInvoker(blocklog.ViewGetRequestReceipt.Message(requestID), blockIndexOrTrieRoot)
	if err != nil || ret == nil {
		return nil, false, err
	}
	rec, err := blocklog.ViewGetRequestReceipt.Output.Decode(ret)
	return rec, rec != nil, err
}

func GetRequestReceiptsForBlock(callViewInvoker CallViewInvoker, blockIndex uint32, blockIndexOrTrieRoot string) ([]*blocklog.RequestReceipt, error) {
	_, res, err := callViewInvoker(
		blocklog.ViewGetRequestReceiptsForBlock.Message(&blockIndex),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return nil, err
	}
	return blocklog.ViewGetRequestReceiptsForBlock.Output2.Decode(res)
}

func IsRequestProcessed(callViewInvoker CallViewInvoker, requestID isc.RequestID, blockIndexOrTrieRoot string) (bool, error) {
	_, ret, err := callViewInvoker(
		blocklog.ViewIsRequestProcessed.Message(requestID),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return false, err
	}
	return blocklog.ViewIsRequestProcessed.Output.Decode(ret)
}

func GetEventsForRequest(callViewInvoker CallViewInvoker, requestID isc.RequestID, blockIndexOrTrieRoot string) ([]*isc.Event, error) {
	_, ret, err := callViewInvoker(
		blocklog.ViewGetEventsForRequest.Message(requestID),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return nil, err
	}
	return blocklog.ViewGetEventsForRequest.Output.Decode(ret)
}

func GetEventsForBlock(callViewInvoker CallViewInvoker, blockIndex uint32, blockIndexOrTrieRoot string) ([]*isc.Event, error) {
	_, ret, err := callViewInvoker(
		blocklog.ViewGetEventsForBlock.Message(&blockIndex),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return nil, err
	}
	return blocklog.ViewGetEventsForBlock.Output2.Decode(ret)
}

func GetEventsForContract(callViewInvoker CallViewInvoker, contractHname isc.Hname, blockIndexOrTrieRoot string) ([]*isc.Event, error) {
	_, ret, err := callViewInvoker(
		blocklog.ViewGetEventsForContract.Message(blocklog.EventsForContractQuery{Contract: contractHname}),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return nil, err
	}
	return blocklog.ViewGetEventsForContract.Output.Decode(ret)
}
