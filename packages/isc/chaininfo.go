// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package isc

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

// ChainInfo is an API structure containing the main parameters of the chain
type ChainInfo struct {
	ChainID         ChainID
	ChainAccountID  iotago.AccountID
	ChainOwnerID    AgentID
	GasFeePolicy    *gas.FeePolicy
	GasLimits       *gas.Limits
	BlockKeepAmount int32

	PublicURL string
	Metadata  *PublicChainMetadata
}
