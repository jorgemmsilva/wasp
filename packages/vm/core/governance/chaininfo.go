// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

// ChainInfo is an API structure which contains main properties of the chain in on place
type ChainInfo struct {
	ChainID         []byte
	ChainOwnerID    []byte
	Description     string
	GasFeePolicy    []byte
	MaxBlobSize     uint32
	MaxEventSize    uint16
	MaxEventsPerReq uint16
}

func (c ChainInfo) Bytes() []byte {
	return util.MustSerialize(c)
}

func FromBytes(data []byte) (ChainInfo, error) {
	return util.Deserialize[ChainInfo](data)
}

func (c ChainInfo) ChainIDDeserialized() (isc.ChainID, error) {
	return isc.ChainIDFromBytes(c.ChainID)
}

func (c ChainInfo) ChainOwnerIDDeserialized() (isc.AgentID, error) {
	return isc.AgentIDFromBytes(c.ChainOwnerID)
}

func (c ChainInfo) GasFeePolicyDeserialized() (*gas.GasFeePolicy, error) {
	return gas.FeePolicyFromBytes(c.GasFeePolicy)
}
