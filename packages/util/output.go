package util

import (
	"errors"

	iotago "github.com/iotaledger/iota.go/v4"
)

func OutputFromBytes(data []byte) (ret iotago.Output, err error) {
	var n int
	n, err = parameters.L1API().Decode(data, &ret)
	if err != nil {
		return nil, err
	}
	if n != len(data) {
		return nil, errors.New("unexpected deserialize size")
	}
	return
}
