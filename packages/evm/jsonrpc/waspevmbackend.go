// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package jsonrpc

import (
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/tracers"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/chainutil"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/trie"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
)

// WaspEVMBackend is the implementation of [ChainBackend] for the production environment.
type WaspEVMBackend struct {
	chain      chaintypes.Chain
	nodePubKey *cryptolib.PublicKey
	tokenInfo  api.InfoResBaseToken
}

var _ ChainBackend = &WaspEVMBackend{}

func NewWaspEVMBackend(ch chaintypes.Chain, nodePubKey *cryptolib.PublicKey, tokenInfo api.InfoResBaseToken) *WaspEVMBackend {
	return &WaspEVMBackend{
		chain:      ch,
		nodePubKey: nodePubKey,
		tokenInfo:  tokenInfo,
	}
}

func (b *WaspEVMBackend) EVMGasRatio(l1API iotago.API) (util.Ratio32, error) {
	// TODO: Cache the gas ratio?
	ret, err := b.ISCCallView(b.ISCLatestState(), governance.Contract.Name, governance.ViewGetEVMGasRatio.Name, nil, l1API)
	if err != nil {
		return util.Ratio32{}, err
	}
	return codec.DecodeRatio32(ret.Get(governance.ParamEVMGasRatio))
}

func (b *WaspEVMBackend) EVMSendTransaction(tx *types.Transaction) error {
	// Ensure the transaction has more gas than the basic Ethereum tx fee.
	intrinsicGas, err := core.IntrinsicGas(tx.Data(), tx.AccessList(), tx.To() == nil, true, true, true)
	if err != nil {
		return err
	}
	if tx.Gas() < intrinsicGas {
		return core.ErrIntrinsicGas
	}

	req, err := isc.NewEVMOffLedgerTxRequest(b.chain.ID(), tx)
	if err != nil {
		return err
	}
	b.chain.Log().Debugf("EVMSendTransaction, evm.tx.nonce=%v, evm.tx.hash=%v => isc.req.id=%v", tx.Nonce(), tx.Hash().Hex(), req.ID())
	if err := b.chain.ReceiveOffLedgerRequest(req, b.nodePubKey); err != nil {
		return fmt.Errorf("tx not added to the mempool: %v", err.Error())
	}

	return nil
}

func (b *WaspEVMBackend) EVMCall(chainOutputs *isc.ChainOutputs, callMsg ethereum.CallMsg, l1API iotago.API) ([]byte, error) {
	return chainutil.EVMCall(b.chain, chainOutputs, callMsg, l1API, b.tokenInfo)
}

func (b *WaspEVMBackend) EVMEstimateGas(chainOutputs *isc.ChainOutputs, callMsg ethereum.CallMsg, l1API iotago.API) (uint64, error) {
	return chainutil.EVMEstimateGas(b.chain, chainOutputs, callMsg, l1API, b.tokenInfo)
}

func (b *WaspEVMBackend) EVMTraceTransaction(
	chainOutputs *isc.ChainOutputs,
	timestamp time.Time,
	iscRequestsInBlock []isc.Request,
	txIndex uint64,
	tracer tracers.Tracer,
	l1API iotago.API,
) error {
	return chainutil.EVMTraceTransaction(
		b.chain,
		chainOutputs,
		timestamp,
		iscRequestsInBlock,
		txIndex,
		tracer,
		l1API,
		b.tokenInfo,
	)
}

func (b *WaspEVMBackend) ISCCallView(
	chainState state.State,
	scName,
	funName string,
	args dict.Dict,
	l1API iotago.API,
) (dict.Dict, error) {
	return chainutil.CallView(chainState, b.chain, isc.Hn(scName), isc.Hn(funName), args, l1API, b.tokenInfo)
}

func (b *WaspEVMBackend) BaseTokenDecimals() uint32 {
	return b.tokenInfo.Decimals
}

func (b *WaspEVMBackend) ISCLatestChainOutputs() (*isc.ChainOutputs, error) {
	latestChainOutputs, err := b.chain.LatestChainOutputs(chaintypes.ActiveOrCommittedState)
	if err != nil {
		return nil, fmt.Errorf("could not get latest ChainOutputs: %w", err)
	}
	return latestChainOutputs, nil
}

func (b *WaspEVMBackend) ISCLatestState() state.State {
	latestState, err := b.chain.LatestState(chaintypes.ActiveOrCommittedState)
	if err != nil {
		panic(fmt.Sprintf("couldn't get latest block index: %s ", err.Error()))
	}
	return latestState
}

func (b *WaspEVMBackend) ISCStateByBlockIndex(blockIndex uint32) (state.State, error) {
	latestState, err := b.chain.LatestState(chaintypes.ActiveOrCommittedState)
	if err != nil {
		return nil, fmt.Errorf("couldn't get latest state: %s", err.Error())
	}
	if latestState.BlockIndex() == blockIndex {
		return latestState, nil
	}
	return b.chain.Store().StateByIndex(blockIndex)
}

func (b *WaspEVMBackend) ISCStateByTrieRoot(trieRoot trie.Hash) (state.State, error) {
	return b.chain.Store().StateByTrieRoot(trieRoot)
}

func (b *WaspEVMBackend) ISCChainID() *isc.ChainID {
	chID := b.chain.ID()
	return &chID
}

var errNotImplemented = errors.New("method not implemented")

func (*WaspEVMBackend) RevertToSnapshot(int) error {
	return errNotImplemented
}

func (*WaspEVMBackend) TakeSnapshot() (int, error) {
	return 0, errNotImplemented
}
