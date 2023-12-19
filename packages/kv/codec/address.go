package codec

import (
	"errors"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
)

func DecodeAddress(b []byte, def ...iotago.Address) (iotago.Address, error) {
	if b == nil {
		if len(def) == 0 {
			return nil, errors.New("cannot decode nil Address")
		}
		return def[0], nil
	}
	if len(b) == 0 {
		return nil, errors.New("invalid Address size")
	}
	return isc.AddressFromBytes(b)
}

func EncodeAddress(addr iotago.Address) []byte {
	return isc.AddressToBytes(addr)
}
