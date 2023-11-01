package isc

import (
	"fmt"
	"math"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

const addressIsNil rwutil.Kind = 0x80

// TODO: copied from iota.go/address.go -- should be made public
func newAddress(addressType iotago.AddressType) (address iotago.Address, err error) {
	switch addressType {
	case iotago.AddressEd25519:
		return &iotago.Ed25519Address{}, nil
	case iotago.AddressAccount:
		return &iotago.AccountAddress{}, nil
	case iotago.AddressNFT:
		return &iotago.NFTAddress{}, nil
	case iotago.AddressAnchor:
		return &iotago.AnchorAddress{}, nil
	case iotago.AddressImplicitAccountCreation:
		return &iotago.ImplicitAccountCreationAddress{}, nil
	case iotago.AddressMulti:
		return &iotago.MultiAddress{}, nil
	case iotago.AddressRestricted:
		return &iotago.RestrictedAddress{}, nil
	default:
		return nil, fmt.Errorf("no handler for address type %d", addressType)
	}
}

func AddressFromReader(rr *rwutil.Reader) (address iotago.Address) {
	kind := rr.ReadKind()
	if kind == addressIsNil {
		return nil
	}
	rr.PushBack().WriteKind(kind)
	address, rr.Err = newAddress(iotago.AddressType(kind))
	rr.ReadSerialized(&address, math.MaxUint16, address.Size())
	return address
}

func AddressToWriter(ww *rwutil.Writer, address iotago.Address) {
	if address == nil {
		ww.WriteKind(addressIsNil)
		return
	}
	ww.WriteSerialized(address, math.MaxUint16, address.Size())
}

func AddressFromBytes(data []byte) (iotago.Address, error) {
	rr := rwutil.NewBytesReader(data)
	return AddressFromReader(rr), rr.Err
}

func AddressToBytes(address iotago.Address) []byte {
	ww := rwutil.NewBytesWriter()
	AddressToWriter(ww, address)
	return ww.Bytes()
}
