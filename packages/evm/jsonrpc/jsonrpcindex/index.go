package jsonrpcindex

import (
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/samber/lo"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/trie"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/evm/emulator"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
)

type Index struct {
	store           kvstore.KVStore
	blockchainDB    func(chainState state.State) *emulator.BlockchainDB
	stateByTrieRoot func(trieRoot trie.Hash) (state.State, error)

	mu sync.Mutex
}

func New(
	blockchainDB func(chainState state.State) *emulator.BlockchainDB,
	stateByTrieRoot func(trieRoot trie.Hash) (state.State, error),
	store kvstore.KVStore,
) *Index {
	return &Index{
		store:           store,
		blockchainDB:    blockchainDB,
		stateByTrieRoot: stateByTrieRoot,
		mu:              sync.Mutex{},
	}
}

func (c *Index) IndexBlock(trieRoot trie.Hash) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, err := c.stateByTrieRoot(trieRoot)
	if err != nil {
		panic(err)
	}
	blockKeepAmount := governance.NewStateReaderFromChainState(state).GetBlockKeepAmount()
	if blockKeepAmount == -1 {
		return // pruning disabled, never cache anything
	}
	// cache the block that will be pruned next (this way reorgs are okay, as long as it never reorgs more than `blockKeepAmount`, which would be catastrophic)
	if state.BlockIndex() < uint32(blockKeepAmount-1) {
		return
	}
	blockIndexToCache := state.BlockIndex() - uint32(blockKeepAmount-1)
	cacheUntil, _ := c.lastBlockIndexed()

	// we need to look at the next block to get the trie commitment of the block we want to cache
	nextBlockInfo, found := blocklog.NewStateReaderFromChainState(state).GetBlockInfo(blockIndexToCache + 1)
	if !found {
		panic(fmt.Errorf("block %d not found on active state %d", blockIndexToCache, state.BlockIndex()))
	}

	// start in the active state of the block to cache
	l1c := lo.Must(transaction.L1CommitmentFromAnchorOutput(nextBlockInfo.PreviousChainOutputs.AnchorOutput))
	activeStateToCache := lo.Must(c.stateByTrieRoot(l1c.TrieRoot()))

	for i := blockIndexToCache; i >= cacheUntil; i-- {
		// walk back and save all blocks between [lastBlockIndexCached...blockIndexToCache]

		blockinfo, found := blocklog.NewStateReaderFromChainState(activeStateToCache).GetBlockInfo(i)
		if !found {
			panic(fmt.Errorf("block %d not found on active state %d", i, state.BlockIndex()))
		}

		db := c.blockchainDB(activeStateToCache)
		blockTrieRoot := activeStateToCache.TrieRoot()
		c.setBlockTrieRootByIndex(i, blockTrieRoot)

		evmBlock := db.GetCurrentBlock()
		c.setBlockIndexByHash(evmBlock.Hash(), i)

		blockTransactions := evmBlock.Transactions()
		for _, tx := range blockTransactions {
			c.setBlockIndexByTxHash(tx.Hash(), i)
		}
		// walk backwards until all blocks are cached
		if i == 0 {
			// nothing more to cache, don't try to walk back further
			break
		}
		l1c := lo.Must(transaction.L1CommitmentFromAnchorOutput(blockinfo.PreviousChainOutputs.AnchorOutput))
		activeStateToCache = lo.Must(c.stateByTrieRoot(l1c.TrieRoot()))
	}
	c.setLastBlockIndexed(blockIndexToCache)
	c.store.Flush()
}

func (c *Index) BlockByNumber(n *big.Int) *types.Block {
	if n == nil {
		return nil
	}
	db := c.evmDBFromBlockIndex(uint32(n.Uint64()))
	if db == nil {
		return nil
	}
	return db.GetBlockByNumber(n.Uint64())
}

func (c *Index) BlockByHash(hash common.Hash) *types.Block {
	blockIndex, ok := c.blockIndexByHash(hash)
	if !ok {
		return nil
	}
	return c.evmDBFromBlockIndex(blockIndex).GetBlockByHash(hash)
}

func (c *Index) BlockTrieRootByIndex(n uint32) *trie.Hash {
	return c.blockTrieRootByIndex(n)
}

func (c *Index) TxByHash(hash common.Hash) (tx *types.Transaction, blockHash common.Hash, blockNumber, txIndex uint64) {
	blockIndex, ok := c.blockIndexByTxHash(hash)
	if !ok {
		return nil, common.Hash{}, 0, 0
	}
	tx, blockHash, blockNumber, txIndex, err := c.evmDBFromBlockIndex(blockIndex).GetTransactionByHash(hash)
	if err != nil {
		panic(err)
	}
	return tx, blockHash, blockNumber, txIndex
}

func (c *Index) GetReceiptByTxHash(hash common.Hash) *types.Receipt {
	blockIndex, ok := c.blockIndexByTxHash(hash)
	if !ok {
		return nil
	}
	return c.evmDBFromBlockIndex(blockIndex).GetReceiptByTxHash(hash)
}

func (c *Index) TxByBlockHashAndIndex(blockHash common.Hash, txIndex uint64) (tx *types.Transaction, blockNumber uint64) {
	blockIndex, ok := c.blockIndexByHash(blockHash)
	if !ok {
		return nil, 0
	}
	block := c.evmDBFromBlockIndex(blockIndex).GetBlockByHash(blockHash)
	if block == nil {
		return nil, 0
	}
	txs := block.Transactions()
	if txIndex > uint64(len(txs)) {
		return nil, 0
	}
	return txs[txIndex], block.NumberU64()
}

func (c *Index) TxByBlockNumberAndIndex(blockNumber *big.Int, txIndex uint64) (tx *types.Transaction, blockHash common.Hash) {
	if blockNumber == nil {
		return nil, common.Hash{}
	}
	db := c.evmDBFromBlockIndex(uint32(blockNumber.Uint64()))
	if db == nil {
		return nil, common.Hash{}
	}
	block := db.GetBlockByHash(blockHash)
	if block == nil {
		return nil, common.Hash{}
	}
	txs := block.Transactions()
	if txIndex > uint64(len(txs)) {
		return nil, common.Hash{}
	}
	return txs[txIndex], block.Hash()
}

// internals

const (
	prefixLastBlockIndexed = iota
	prefixBlockTrieRootByIndex
	prefixBlockIndexByTxHash
	prefixBlockIndexByHash
)

func keyLastBlockIndexed() kvstore.Key {
	return []byte{prefixLastBlockIndexed}
}

func keyBlockTrieRootByIndex(i uint32) kvstore.Key {
	key := []byte{prefixBlockTrieRootByIndex}
	key = append(key, codec.Uint32.Encode(i)...)
	return key
}

func keyBlockIndexByTxHash(hash common.Hash) kvstore.Key {
	key := []byte{prefixBlockIndexByTxHash}
	key = append(key, hash[:]...)
	return key
}

func keyBlockIndexByHash(hash common.Hash) kvstore.Key {
	key := []byte{prefixBlockIndexByHash}
	key = append(key, hash[:]...)
	return key
}

func (c *Index) get(key kvstore.Key) []byte {
	ret, err := c.store.Get(key)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil
		}
		panic(err)
	}
	return ret
}

func (c *Index) set(key kvstore.Key, value []byte) {
	err := c.store.Set(key, value)
	if err != nil {
		panic(err)
	}
}

func (c *Index) setLastBlockIndexed(n uint32) {
	c.set(keyLastBlockIndexed(), codec.Uint32.Encode(n))
}

func (c *Index) lastBlockIndexed() (uint32, bool) {
	bytes := c.get(keyLastBlockIndexed())
	if bytes == nil {
		return 0, false
	}
	return lo.Must(codec.Uint32.Decode(bytes)), true
}

func (c *Index) setBlockTrieRootByIndex(i uint32, hash trie.Hash) {
	c.set(keyBlockTrieRootByIndex(i), hash.Bytes())
}

func (c *Index) blockTrieRootByIndex(i uint32) *trie.Hash {
	bytes := c.get(keyBlockTrieRootByIndex(i))
	if bytes == nil {
		return nil
	}
	hash, err := trie.HashFromBytes(bytes)
	if err != nil {
		panic(err)
	}
	return &hash
}

func (c *Index) setBlockIndexByTxHash(txHash common.Hash, blockIndex uint32) {
	c.set(keyBlockIndexByTxHash(txHash), codec.Uint32.Encode(blockIndex))
}

func (c *Index) blockIndexByTxHash(txHash common.Hash) (uint32, bool) {
	bytes := c.get(keyBlockIndexByTxHash(txHash))
	if bytes == nil {
		return 0, false
	}
	return lo.Must(codec.Uint32.Decode(bytes)), true
}

func (c *Index) setBlockIndexByHash(hash common.Hash, blockIndex uint32) {
	c.set(keyBlockIndexByHash(hash), codec.Uint32.Encode(blockIndex))
}

func (c *Index) blockIndexByHash(hash common.Hash) (uint32, bool) {
	bytes := c.get(keyBlockIndexByHash(hash))
	if bytes == nil {
		return 0, false
	}
	return lo.Must(codec.Uint32.Decode(bytes)), true
}

func (c *Index) evmDBFromBlockIndex(n uint32) *emulator.BlockchainDB {
	trieRoot := c.blockTrieRootByIndex(n)
	if trieRoot == nil {
		return nil
	}
	state, err := c.stateByTrieRoot(*trieRoot)
	if err != nil {
		panic(err)
	}
	return c.blockchainDB(state)
}
