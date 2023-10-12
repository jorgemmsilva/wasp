package codec

import (
	"errors"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/parameters"
)

func DecodeOutput(b []byte) (out iotago.TxEssenceOutput, err error) {
	n, err := parameters.L1API().Decode(b, &out)
	if err != nil {
		return nil, err
	}
	if n != len(b) {
		return nil, errors.New("incomplete read")
	}
	return
}

func MustDecodeOutput(b []byte) iotago.TxEssenceOutput {
	o, err := DecodeOutput(b)
	if err != nil {
		panic(err)
	}
	return o
}

func EncodeOutput(out iotago.TxEssenceOutput) []byte {
	b, err := parameters.L1API().Encode(out)
	if err != nil {
		panic(err)
	}
	return b
}
