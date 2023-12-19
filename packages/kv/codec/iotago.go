package codec

import (
	"errors"

	iotago "github.com/iotaledger/iota.go/v4"
)

func DecodeOutput(b []byte, l1API iotago.API) (out iotago.TxEssenceOutput, err error) {
	n, err := l1API.Decode(b, &out)
	if err != nil {
		return nil, err
	}
	if n != len(b) {
		return nil, errors.New("incomplete read")
	}
	return
}

func EncodeOutput(out iotago.TxEssenceOutput, l1API iotago.API) []byte {
	b, err := l1API.Encode(out)
	if err != nil {
		panic(err)
	}
	return b
}
