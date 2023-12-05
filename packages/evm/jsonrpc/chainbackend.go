// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package jsonrpc

import (
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/tracers"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/trie"
)

// ChainBackend provides access to the underlying ISC chain.
type ChainBackend interface {
	EVMSendTransaction(tx *types.Transaction) error
	EVMCall(chainOutputs *isc.ChainOutputs, callMsg ethereum.CallMsg, l1API iotago.API) ([]byte, error)
	EVMEstimateGas(chainOutputs *isc.ChainOutputs, callMsg ethereum.CallMsg, l1API iotago.API) (uint64, error)
	EVMTraceTransaction(
		chainOutputs *isc.ChainOutputs,
		timestamp time.Time,
		iscRequestsInBlock []isc.Request,
		txIndex uint64,
		tracer tracers.Tracer,
		l1API iotago.API,
	) error
	ISCChainID() *isc.ChainID
	ISCCallView(chainState state.State, scName string, funName string, args dict.Dict, l1API iotago.API) (dict.Dict, error)
	ISCLatestChainOutputs() (*isc.ChainOutputs, error)
	ISCLatestState() state.State
	ISCStateByBlockIndex(blockIndex uint32) (state.State, error)
	ISCStateByTrieRoot(trieRoot trie.Hash) (state.State, error)
	BaseTokenDecimals() uint32
	TakeSnapshot() (int, error)
	RevertToSnapshot(int) error
}
