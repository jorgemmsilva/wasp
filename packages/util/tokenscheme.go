package util

import iotago "github.com/iotaledger/iota.go/v4"

func MustTokenScheme(tokenScheme iotago.TokenScheme) *iotago.SimpleTokenScheme {
	simpleTokenScheme, ok := tokenScheme.(*iotago.SimpleTokenScheme)
	if !ok {
		panic("unrecognized token scheme")
	}
	return simpleTokenScheme
}
