package isc

import (
	"fmt"
	"io"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

// ContractAgentID is an AgentID formed by a ChainID and a contract Hname.
type ContractAgentID struct {
	chainID ChainID
	hname   Hname
}

var _ AgentIDWithL1Address = &ContractAgentID{}

func NewContractAgentID(chainID ChainID, hname Hname) *ContractAgentID {
	return &ContractAgentID{chainID: chainID, hname: hname}
}

func contractAgentIDFromString(hnamePart, addrPart string, expectedPrefix iotago.NetworkPrefix) (AgentID, error) {
	chainID, err := ChainIDFromBech32(addrPart, expectedPrefix)
	if err != nil {
		return nil, fmt.Errorf("AgentIDFromBech32: %w", err)
	}
	h, err := HnameFromString(hnamePart)
	if err != nil {
		return nil, fmt.Errorf("AgentIDFromBech32: %w", err)
	}
	return NewContractAgentID(chainID, h), nil
}

func (a *ContractAgentID) Address() iotago.Address {
	return a.chainID.AsAddress()
}

func (a *ContractAgentID) Bytes() []byte {
	return rwutil.WriteToBytes(a)
}

func (a *ContractAgentID) ChainID() ChainID {
	return a.chainID
}

func (a *ContractAgentID) BelongsToChain(cID ChainID) bool {
	return a.chainID.Equals(cID)
}

func (a *ContractAgentID) BytesWithoutChainID() []byte {
	ww := rwutil.NewBytesWriter()
	ww.Write(&a.hname)
	return ww.Bytes()
}

func (a *ContractAgentID) Equals(other AgentID) bool {
	if other == nil {
		return false
	}
	if other.Kind() != a.Kind() {
		return false
	}
	o := other.(*ContractAgentID)
	return o.chainID.Equals(a.chainID) && o.hname == a.hname
}

func (a *ContractAgentID) Hname() Hname {
	return a.hname
}

func (a *ContractAgentID) Kind() AgentIDKind {
	return AgentIDKindContract
}

func (a *ContractAgentID) Bech32(prefix iotago.NetworkPrefix) string {
	return a.hname.String() + AgentIDStringSeparator + a.chainID.Bech32(prefix)
}

func (a *ContractAgentID) Read(r io.Reader) error {
	rr := rwutil.NewReader(r)
	rr.ReadKindAndVerify(rwutil.Kind(a.Kind()))
	rr.Read(&a.chainID)
	rr.Read(&a.hname)
	return rr.Err
}

func (a *ContractAgentID) Write(w io.Writer) error {
	ww := rwutil.NewWriter(w)
	ww.WriteKind(rwutil.Kind(a.Kind()))
	ww.Write(&a.chainID)
	ww.Write(&a.hname)
	return ww.Err
}
