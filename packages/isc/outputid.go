package isc

import (
	iotago "github.com/iotaledger/iota.go/v4"
)

const Million = iotago.BaseToken(1_000_000)

var emptyOutputID = iotago.OutputID{}

func IsEmptyOutputID(outputID iotago.OutputID) bool {
	return outputID == emptyOutputID
}
