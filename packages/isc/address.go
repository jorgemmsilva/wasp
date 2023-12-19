package isc

import (
	"fmt"
	"math"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

// TODO: copied from iota.go/address.go -- should probably be made public in iotago
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

const addressIsNil rwutil.Kind = 0x80

func AddressFromReader(rr *rwutil.Reader) iotago.Address {
	kind := rr.ReadKind()
	if kind == addressIsNil {
		return nil
	}
	if rr.Err != nil {
		return nil
	}
	var address iotago.Address
	address, rr.Err = newAddress(iotago.AddressType(kind))
	if rr.Err != nil {
		return nil
	}
	rr.PushBack().WriteKind(kind)
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

func AddressFromString(prefix iotago.NetworkPrefix, s string) (iotago.Address, error) {
	p, addr, err := iotago.ParseBech32(s)
	if err != nil {
		return nil, err
	}
	if p != prefix {
		return nil, fmt.Errorf("expected network prefix %q, got %q", prefix, p)
	}
	return addr, nil
}
