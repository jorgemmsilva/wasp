package isc

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

func NativeTokenIDFromBytes(data []byte) (ret iotago.NativeTokenID, err error) {
	rr := rwutil.NewBytesReader(data)
	rr.ReadN(ret[:])
	rr.Close()
	return ret, rr.Err
}

func NativeTokenIDToBytes(tokenID iotago.NativeTokenID) []byte {
	return tokenID[:]
}
