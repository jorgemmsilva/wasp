package codec

import (
	"context"
	"errors"

	iotago "github.com/iotaledger/iota.go/v4"
)

func DecodeOutput(b []byte) (out iotago.TxEssenceOutput, err error) {
	n, err := iotago.CommonSerixAPI().Decode(context.Background(), b, &out)
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
	b, err := iotago.CommonSerixAPI().Encode(context.Background(), out)
	if err != nil {
		panic(err)
	}
	return b
}
