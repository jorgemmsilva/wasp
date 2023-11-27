package codec

import (
	"errors"

	iotago "github.com/iotaledger/iota.go/v4"
)

func DecodeTokenScheme(b []byte, l1API iotago.API, def ...iotago.TokenScheme) (ts iotago.TokenScheme, err error) {
	if len(b) == 0 {
		if len(def) > 0 {
			return def[0], nil
		}
		return nil, errors.New("wrong data length")
	}
	n, err := l1API.Decode(b, &ts)
	if err != nil {
		return nil, err
	}
	if n != len(b) {
		return nil, errors.New("incomplete read")
	}
	return
}

func MustDecodeTokenScheme(b []byte, l1API iotago.API, def ...iotago.TokenScheme) iotago.TokenScheme {
	t, err := DecodeTokenScheme(b, l1API, def...)
	if err != nil {
		panic(err)
	}
	return t
}

func EncodeTokenScheme(value iotago.TokenScheme, l1API iotago.API) []byte {
	b, err := l1API.Encode(value)
	if err != nil {
		panic(err)
	}
	return b
}
