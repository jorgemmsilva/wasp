// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"github.com/iotaledger/wasp/packages/chain/consensus/journal"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/tcrypto"
)

type Provider func() Registry

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
	SaveChainRecord(rec *ChainRecord) error
}

type Registry interface {
	NodeIdentityProvider
	tcrypto.DKShareRegistryProvider
	ChainRecordRegistryProvider
	journal.Registry
	peering.TrustedNetworkManager
}
