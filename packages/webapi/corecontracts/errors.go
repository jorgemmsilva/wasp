package corecontracts

import (
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/vm/core/errors"
)

func ErrorMessageFormat(callViewInvoker CallViewInvoker, contractID isc.Hname, errorID uint16, blockIndexOrTrieRoot string) (string, error) {
	errorCode := isc.NewVMErrorCode(contractID, errorID)
	_, ret, err := callViewInvoker(
		errors.ViewGetErrorMessageFormat.Message(errorCode),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return "", err
	}
	return errors.ViewGetErrorMessageFormat.Output.Decode(ret)
}
