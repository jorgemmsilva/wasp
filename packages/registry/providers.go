// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
)

type Provider func() *Impl

type NodeIdentityProvider interface {
	GetNodeIdentity() *cryptolib.KeyPair
	GetNodePublicKey() *cryptolib.PublicKey
}

// ChainRecordRegistryProvider stands for a partial registry interface, needed for this package.
type ChainRecordRegistryProvider interface {
	GetChainRecordByChainID(chainID *isc.ChainID) (*ChainRecord, error)
	GetChainRecords() ([]*ChainRecord, error)
	UpdateChainRecord(chainID *isc.ChainID, f func(*ChainRecord) bool) (*ChainRecord, error)
	ActivateChainRecord(chainID *isc.ChainID) (*ChainRecord, error)
	DeactivateChainRecord(chainID *isc.ChainID) (*ChainRecord, error)
}
