package isc

import (
	"fmt"
	"io"

	iotago "github.com/iotaledger/iota.go/v4"

	"github.com/iotaledger/wasp/packages/util/rwutil"
)

// AddressAgentID is an AgentID backed by a non-anchor address.
type AddressAgentID struct {
	a iotago.Address
}

var _ AgentIDWithL1Address = &AddressAgentID{}

func NewAddressAgentID(addr iotago.Address) *AddressAgentID {
	return &AddressAgentID{a: addr}
}

func addressAgentIDFromString(s string, expectedPrefix iotago.NetworkPrefix) (*AddressAgentID, error) {
	prefix, addr, err := iotago.ParseBech32(s)
	if err != nil {
		return nil, err
	}
	if prefix != expectedPrefix {
		return nil, fmt.Errorf("expected network prefix %s, got %s", expectedPrefix, prefix)
	}
	return &AddressAgentID{a: addr}, nil
}

func (a *AddressAgentID) Address() iotago.Address {
	return a.a
}

func (a *AddressAgentID) Bytes() []byte {
	return rwutil.WriteToBytes(a)
}

func (a *AddressAgentID) BelongsToChain(ChainID) bool {
	return false
}

func (a *AddressAgentID) BytesWithoutChainID() []byte {
	return a.Bytes()
}

func (a *AddressAgentID) Equals(other AgentID) bool {
	if other == nil {
		return false
	}
	if other.Kind() != a.Kind() {
		return false
	}
	return other.(*AddressAgentID).a.Equal(a.a)
}

func (a *AddressAgentID) Kind() AgentIDKind {
	return AgentIDKindAddress
}

func (a *AddressAgentID) Bech32(prefix iotago.NetworkPrefix) string {
	return a.a.Bech32(prefix)
}

func (a *AddressAgentID) Read(r io.Reader) error {
	rr := rwutil.NewReader(r)
	rr.ReadKindAndVerify(rwutil.Kind(a.Kind()))
	a.a = AddressFromReader(rr)
	return rr.Err
}

func (a *AddressAgentID) Write(w io.Writer) error {
	ww := rwutil.NewWriter(w)
	ww.WriteKind(rwutil.Kind(a.Kind()))
	AddressToWriter(ww, a.a)
	return ww.Err
}
