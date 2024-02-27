// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package chain

import (
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/state"
)

////////////////////////////////////////////////////////////////////////////////
// emptyChainListener

type emptyChainListener struct{}

var _ chaintypes.ChainListener = &emptyChainListener{}

func NewEmptyChainListener() chaintypes.ChainListener {
	return &emptyChainListener{}
}

func (ecl *emptyChainListener) BlockApplied(chainID isc.ChainID, block state.Block, latestState kv.KVStoreReader) {
}
func (ecl *emptyChainListener) AccessNodesUpdated(isc.ChainID, []*cryptolib.PublicKey) {}
func (ecl *emptyChainListener) ServerNodesUpdated(isc.ChainID, []*cryptolib.PublicKey) {}
