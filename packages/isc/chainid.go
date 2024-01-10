// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package isc

import (
	"fmt"
	"io"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/hashing"

	"github.com/iotaledger/wasp/packages/util/rwutil"
)

const ChainIDLength = iotago.AnchorIDLength

var emptyChainID = ChainID{}

// ChainID represents the global identifier of the chain
// It is wrapped AnchorAddress, an address without a private key behind
type (
	ChainID    iotago.AnchorID
	ChainIDKey string
)

// EmptyChainID returns an empty ChainID.
func EmptyChainID() ChainID {
	return emptyChainID
}

func ChainIDFromAddress(addr *iotago.AnchorAddress) ChainID {
	return ChainIDFromAnchorID(addr.AnchorID())
}

// ChainIDFromAnchorID creates new chain ID from anchor address
func ChainIDFromAnchorID(anchorID iotago.AnchorID) ChainID {
	return ChainID(anchorID)
}

// ChainIDFromBytes reconstructs a ChainID from its binary representation.
func ChainIDFromBytes(data []byte) (ret ChainID, err error) {
	_, err = rwutil.ReadFromBytes(data, &ret)
	return ret, err
}

func ChainIDFromBech32(bech32 string) (ChainID, error) {
	_, addr, err := iotago.ParseBech32(bech32)
	if err != nil {
		return ChainID{}, err
	}
	if addr.Type() != iotago.AddressAnchor {
		return ChainID{}, fmt.Errorf("chainID must be an anchor address (%s)", bech32)
	}
	return ChainIDFromAddress(addr.(*iotago.AnchorAddress)), nil
}

func ChainIDFromString(s string) (ChainID, error) {
	chID, err := iotago.AnchorIDFromHexString(s)
	return ChainID(chID), err
}

func ChainIDFromKey(key ChainIDKey) ChainID {
	chainID, err := ChainIDFromString(string(key))
	if err != nil {
		panic(err)
	}
	return chainID
}

// RandomChainID creates a random chain ID. Used for testing only
func RandomChainID(seed ...[]byte) ChainID {
	var h hashing.HashValue
	if len(seed) > 0 {
		h = hashing.HashData(seed[0])
	} else {
		h = hashing.PseudoRandomHash(nil)
	}
	chainID, err := ChainIDFromBytes(h[:ChainIDLength])
	if err != nil {
		panic(err)
	}
	return chainID
}

func (id ChainID) AsAddress() iotago.Address {
	addr := iotago.AnchorAddress(id)
	return &addr
}

func (id ChainID) AsAnchorAddress() iotago.AnchorAddress {
	return iotago.AnchorAddress(id)
}

func (id ChainID) AsAnchorID() iotago.AnchorID {
	return iotago.AnchorID(id)
}

func (id ChainID) Bytes() []byte {
	return id[:]
}

func (id ChainID) Empty() bool {
	return id == emptyChainID
}

func (id ChainID) Equals(other ChainID) bool {
	return id == other
}

func (id ChainID) Key() ChainIDKey {
	return ChainIDKey(id.AsAnchorID().String())
}

func (id ChainID) IsSameChain(agentID AgentID) bool {
	contract, ok := agentID.(*ContractAgentID)
	if !ok {
		return false
	}
	return id.Equals(contract.ChainID())
}

func (id ChainID) ShortString() string {
	return id.AsAddress().String()[0:10]
}

// String human-readable form (bech32)
func (id ChainID) Bech32(bech32HRP iotago.NetworkPrefix) string {
	return id.AsAddress().Bech32(bech32HRP)
}

func (id ChainID) String() string {
	return id.AsAnchorID().ToHex()
}

func (id *ChainID) Read(r io.Reader) error {
	return rwutil.ReadN(r, id[:])
}

func (id *ChainID) Write(w io.Writer) error {
	return rwutil.WriteN(w, id[:])
}
