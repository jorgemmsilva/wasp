package codec

import (
	"errors"

	"github.com/iotaledger/wasp/packages/isc"
)

func DecodeVMErrorCode(b []byte, def ...isc.VMErrorCode) (ret isc.VMErrorCode, err error) {
	if b == nil {
		if len(def) == 0 {
			return ret, errors.New("cannot decode nil VMErrorCode")
		}
		return def[0], nil
	}
	return isc.VMErrorCodeFromBytes(b)
}

func EncodeVMErrorCode(code isc.VMErrorCode) []byte {
	return code.Bytes()
}
