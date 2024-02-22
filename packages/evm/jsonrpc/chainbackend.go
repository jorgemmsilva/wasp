// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package jsonrpc

import (
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/tracers"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/trie"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

// ChainBackend provides access to the underlying ISC chain.
type ChainBackend interface {
	EVMSendTransaction(tx *types.Transaction) error
	EVMCall(chainOutputs *isc.ChainOutputs, callMsg ethereum.CallMsg) ([]byte, error)
	EVMEstimateGas(chainOutputs *isc.ChainOutputs, callMsg ethereum.CallMsg) (uint64, error)
	EVMTraceTransaction(
		chainOutputs *isc.ChainOutputs,
		timestamp time.Time,
		iscRequestsInBlock []isc.Request,
		txIndex uint64,
		tracer tracers.Tracer,
	) error
	FeePolicy(blockIndex uint32) (*gas.FeePolicy, error)
	ISCChainID() *isc.ChainID
	ISCCallView(chainState state.State, msg isc.Message) (dict.Dict, error)
	ISCLatestChainOutputs() (*isc.ChainOutputs, error)
	ISCLatestState() state.State
	ISCStateByBlockIndex(blockIndex uint32) (state.State, error)
	ISCStateByTrieRoot(trieRoot trie.Hash) (state.State, error)
	L1APIProvider() iotago.APIProvider
	BaseTokenInfo() *api.InfoResBaseToken
	TakeSnapshot() (int, error)
	RevertToSnapshot(int) error
}
