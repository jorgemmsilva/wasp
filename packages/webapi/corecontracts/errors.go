package corecontracts

import (
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/kvdecoder"
	"github.com/iotaledger/wasp/packages/vm/core/errors"
)

func ErrorMessageFormat(callViewInvoker CallViewInvoker, contractID isc.Hname, errorID uint16, blockIndexOrTrieRoot string) (string, error) {
	errorCode := isc.NewVMErrorCode(contractID, errorID)

	_, ret, err := callViewInvoker(
		errors.Contract.Hname(),
		errors.ViewGetErrorMessageFormat.Hname(),
		codec.MakeDict(map[string]interface{}{
			errors.ParamErrorCode: errorCode.Bytes(),
		}),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return "", err
	}

	resultDecoder := kvdecoder.New(ret)
	messageFormat, err := resultDecoder.GetString(errors.ParamErrorMessageFormat)
	if err != nil {
		return "", err
	}

	return messageFormat, nil
}
