package mempooltypes

import (
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/state"
)

// Partial interface for providing chain events to the outside.
// This interface is in the mempool part only because it tracks
// the actual state for checking the consumed requests.
type ChainListener interface {
	// This function is called by the chain when new block is applied to the
	// state. This block might be not confirmed yet, but the chain is going
	// to build the next block on top of this one.
	BlockApplied(chainID isc.ChainID, block state.Block, latestState kv.KVStoreReader)
}
