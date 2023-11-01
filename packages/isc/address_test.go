package isc_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
)

func TestAddressSerialization(t *testing.T) {
	{
		data := make([]byte, iotago.Ed25519AddressSerializedBytesSize)
		data[0] = byte(iotago.AddressEd25519)
		rand.Read(data[1:])

		addr1, err := isc.AddressFromBytes(data)
		require.NoError(t, err)
		require.IsType(t, &iotago.Ed25519Address{}, addr1)

		data2 := isc.AddressToBytes(addr1)
		require.Equal(t, data, data2)
		addr2, err := isc.AddressFromBytes(data2)
		require.NoError(t, err)
		require.Equal(t, addr1, addr2)
	}
	{
		data := make([]byte, iotago.AnchorAddressSerializedBytesSize)
		data[0] = byte(iotago.AddressAnchor)
		rand.Read(data[1:])

		addr1, err := isc.AddressFromBytes(data)
		require.NoError(t, err)
		require.IsType(t, &iotago.AnchorAddress{}, addr1)

		data2 := isc.AddressToBytes(addr1)
		require.Equal(t, data, data2)
		addr2, err := isc.AddressFromBytes(data2)
		require.NoError(t, err)
		require.Equal(t, addr1, addr2)
	}
	{
		data := make([]byte, iotago.NFTAddressSerializedBytesSize)
		data[0] = byte(iotago.AddressNFT)
		rand.Read(data[1:])

		addr1, err := isc.AddressFromBytes(data)
		require.NoError(t, err)
		require.IsType(t, &iotago.NFTAddress{}, addr1)

		data2 := isc.AddressToBytes(addr1)
		require.Equal(t, data, data2)
		addr2, err := isc.AddressFromBytes(data2)
		require.NoError(t, err)
		require.Equal(t, addr1, addr2)
	}
}
