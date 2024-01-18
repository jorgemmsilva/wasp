// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package isc

import (
	"errors"
	"io"
	"strings"

	iotago "github.com/iotaledger/iota.go/v4"

	"github.com/iotaledger/wasp/packages/util/rwutil"
)

type AgentIDKind rwutil.Kind

const (
	AgentIDKindNil AgentIDKind = iota
	AgentIDKindAddress
	AgentIDKindContract
	AgentIDKindEthereumAddress

	AgentIDIsNil AgentIDKind = 0x80
)

const AgentIDStringSeparator = "@"

// AgentID represents any entity that can hold assets on L2 and/or call contracts.
type AgentID interface {
	Bytes() []byte
	BelongsToChain(ChainID) bool
	BytesWithoutChainID() []byte
	Equals(other AgentID) bool
	Kind() AgentIDKind
	Read(r io.Reader) error
	// Bech32 returns a user-facing string representation
	Bech32(prefix iotago.NetworkPrefix) string
	Write(w io.Writer) error
}

// AgentIDWithL1Address is an AgentID backed by an L1 address (either AddressAgentID or ContractAgentID).
type AgentIDWithL1Address interface {
	AgentID
	Address() iotago.Address
}

// AddressFromAgentID returns the L1 address of the AgentID, if applicable.
func AddressFromAgentID(a AgentID) (iotago.Address, bool) {
	wa, ok := a.(AgentIDWithL1Address)
	if !ok {
		return nil, false
	}
	return wa.Address(), true
}

// HnameFromAgentID returns the hname of the AgentID, or HnameNil if not applicable.
func HnameFromAgentID(a AgentID) Hname {
	if ca, ok := a.(*ContractAgentID); ok {
		return ca.Hname()
	}
	return HnameNil
}

// NewAgentID creates an AddressAgentID if the address is not an AnchorAddress;
// otherwise a ContractAgentID with hname = HnameNil.
func NewAgentID(addr iotago.Address) AgentID {
	if addr.Type() == iotago.AddressAnchor {
		chainID := ChainIDFromAddress(addr.(*iotago.AnchorAddress))
		return NewContractAgentID(chainID, HnameNil)
	}
	return NewAddressAgentID(addr)
}

func AgentIDFromBytes(data []byte) (AgentID, error) {
	rr := rwutil.NewBytesReader(data)
	return AgentIDFromReader(rr), rr.Err
}

func AgentIDFromReader(rr *rwutil.Reader) (ret AgentID) {
	kind := rr.ReadKind()
	switch AgentIDKind(kind) {
	case AgentIDIsNil:
		return nil
	case AgentIDKindNil:
		ret = new(NilAgentID)
	case AgentIDKindAddress:
		ret = new(AddressAgentID)
	case AgentIDKindContract:
		ret = new(ContractAgentID)
	case AgentIDKindEthereumAddress:
		ret = new(EthereumAddressAgentID)
	default:
		if rr.Err == nil {
			rr.Err = errors.New("invalid AgentID kind")
			return nil
		}
	}
	rr.PushBack().WriteKind(kind)
	rr.Read(ret)
	return ret
}

func AgentIDToWriter(ww *rwutil.Writer, agent AgentID) {
	if agent == nil {
		ww.WriteKind(rwutil.Kind(AgentIDIsNil))
		return
	}
	ww.Write(agent)
}

// AgentIDFromBech32 parses the human-readable string representation
func AgentIDFromBech32(s string, expectedPrefix iotago.NetworkPrefix) (AgentID, error) {
	if s == nilAgentIDString {
		return &NilAgentID{}, nil
	}
	var contractPart, addrPart string
	{
		parts := strings.Split(s, AgentIDStringSeparator)
		switch len(parts) {
		case 1:
			addrPart = parts[0]
		case 2:
			addrPart = parts[1]
			contractPart = parts[0]
		default:
			return nil, errors.New("invalid AgentID format")
		}
	}

	if contractPart != "" {
		if strings.HasPrefix(contractPart, "0x") {
			return ethAgentIDFromString(contractPart, addrPart, expectedPrefix)
		}
		return contractAgentIDFromString(contractPart, addrPart, expectedPrefix)
	}
	return addressAgentIDFromString(s, expectedPrefix)
}

// NewRandomAgentID creates random AgentID
func NewRandomAgentID() AgentID {
	return NewContractAgentID(RandomChainID(), Hn("testName"))
}
