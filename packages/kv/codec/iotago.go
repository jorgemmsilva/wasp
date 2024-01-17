package codec

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
)

var Address = NewCodec(decodeAddress, isc.AddressToBytes)

func decodeAddress(b []byte) (iotago.Address, error) {
	if len(b) == 0 {
		return nil, errors.New("invalid Address size")
	}
	return isc.AddressFromBytes(b)
}

var (
	// using hardcoded L1API, assuming that serializatin format does not change over time
	l1API = iotago.V3API(iotago.NewV3SnapshotProtocolParameters(iotago.WithVersion(3)))

	Output      = newIotagoCodec[iotago.TxEssenceOutput](l1API)
	TokenScheme = newIotagoCodec[iotago.TokenScheme](l1API)
)

func newIotagoCodec[T any](l1API iotago.API) Codec[T] {
	return NewCodec(
		func(b []byte) (v T, err error) {
			n, err := l1API.Decode(b, &v)
			if err != nil {
				return
			}
			if n != len(b) {
				err = errors.New("incomplete read")
			}
			return
		},
		func(v T) []byte {
			return lo.Must(l1API.Encode(v))
		},
	)
}

var NativeTokenID = NewCodec(decodeNativeTokenID, encodeNativeTokenID)

func decodeNativeTokenID(b []byte) (ret iotago.NativeTokenID, err error) {
	if len(b) != len(ret) {
		return ret, fmt.Errorf("%T: bytes length must be %d", ret, len(ret))
	}
	copy(ret[:], b)
	return ret, nil
}

func encodeNativeTokenID(nftID iotago.NativeTokenID) []byte {
	return nftID[:]
}

var NFTID = NewCodec(decodeNFTID, encodeNFTID)

func decodeNFTID(b []byte) (ret iotago.NFTID, err error) {
	if len(b) != len(ret) {
		return ret, fmt.Errorf("%T: bytes length must be %d", ret, len(ret))
	}
	copy(ret[:], b)
	return ret, nil
}

func encodeNFTID(nftID iotago.NFTID) []byte {
	return nftID[:]
}

var BaseToken = newIntCodec[iotago.BaseToken](8, binary.LittleEndian.Uint64, binary.LittleEndian.PutUint64)
