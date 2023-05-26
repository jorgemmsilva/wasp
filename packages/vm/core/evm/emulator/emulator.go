// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package emulator

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/params"
	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/iotaledger/wasp/packages/evm/evmutil"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/subrealm"
	"github.com/iotaledger/wasp/packages/util/panicutil"
	"github.com/iotaledger/wasp/packages/vm/gas"
	"github.com/iotaledger/wasp/packages/vm/vmcontext/vmexceptions"
)

type EVMEmulator struct {
	timestamp       uint64
	gasLimits       GasLimits
	blockKeepAmount int32
	chainConfig     *params.ChainConfig
	kv              kv.KVStore
	vmConfig        vm.Config
	l2Balance       L2Balance
}

type GasLimits struct {
	Block uint64
	Call  uint64
}

var configCache *lru.Cache[int, *params.ChainConfig]

func init() {
	var err error
	configCache, err = lru.New[int, *params.ChainConfig](100)
	if err != nil {
		panic(err)
	}
}

func getConfig(chainID int) *params.ChainConfig {
	if c, ok := configCache.Get(chainID); ok {
		return c
	}
	c := &params.ChainConfig{
		ChainID:             big.NewInt(int64(chainID)),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		MuirGlacierBlock:    big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
		Ethash:              &params.EthashConfig{},
		ShanghaiTime:        new(uint64),
	}
	configCache.Add(chainID, c)
	return c
}

const (
	KeyStateDB      = "s"
	KeyBlockchainDB = "b"
)

func newStateDB(store kv.KVStore, l2Balance L2Balance) *StateDB {
	return NewStateDB(subrealm.New(store, KeyStateDB), l2Balance)
}

func NewBlockchainDBSubrealm(store kv.KVStore) kv.KVStore {
	return subrealm.New(store, KeyBlockchainDB)
}

func newBlockchainDBWithSubrealm(store kv.KVStore, blockGasLimit uint64, blockKeepAmount int32) *BlockchainDB {
	return NewBlockchainDB(NewBlockchainDBSubrealm(store), blockGasLimit, blockKeepAmount)
}

// Init initializes the EVM state with the provided genesis allocation parameters
func Init(
	store kv.KVStore,
	chainID uint16,
	gasLimits GasLimits,
	timestamp uint64,
	alloc core.GenesisAlloc,
) {
	bdb := newBlockchainDBWithSubrealm(store, gasLimits.Block, BlockKeepAll)
	if bdb.Initialized() {
		panic("evm state already initialized in kvstore")
	}
	bdb.Init(chainID, timestamp)

	statedb := newStateDB(store, nil)
	for addr, account := range alloc {
		statedb.CreateAccount(addr)
		if account.Balance != nil {
			panic("balances must be 0 at genesis")
		}
		if account.Code != nil {
			statedb.SetCode(addr, account.Code)
		}
		for k, v := range account.Storage {
			statedb.SetState(addr, k, v)
		}
		statedb.SetNonce(addr, account.Nonce)
	}
}

func NewEVMEmulator(
	store kv.KVStore,
	timestamp uint64,
	gasLimits GasLimits,
	blockKeepAmount int32,
	magicContracts map[common.Address]vm.ISCMagicContract,
	l2Balance L2Balance,
) *EVMEmulator {
	bdb := newBlockchainDBWithSubrealm(store, gasLimits.Block, blockKeepAmount)
	if !bdb.Initialized() {
		panic("must initialize genesis block first")
	}

	return &EVMEmulator{
		timestamp:       timestamp,
		gasLimits:       gasLimits,
		blockKeepAmount: blockKeepAmount,
		chainConfig:     getConfig(int(bdb.GetChainID())),
		kv:              store,
		vmConfig:        vm.Config{MagicContracts: magicContracts},
		l2Balance:       l2Balance,
	}
}

func (e *EVMEmulator) StateDB() *StateDB {
	return newStateDB(e.kv, e.l2Balance)
}

func (e *EVMEmulator) BlockchainDB() *BlockchainDB {
	return newBlockchainDBWithSubrealm(e.kv, e.gasLimits.Block, e.blockKeepAmount)
}

func (e *EVMEmulator) BlockGasLimit() uint64 {
	return e.gasLimits.Block
}

func (e *EVMEmulator) CallGasLimit() uint64 {
	return e.gasLimits.Call
}

func (e *EVMEmulator) ChainContext() core.ChainContext {
	return &chainContext{
		engine: ethash.NewFaker(),
	}
}

func coreMsgFromCallMsg(call ethereum.CallMsg, statedb *StateDB) *core.Message {
	return &core.Message{
		To:                call.To,
		From:              call.From,
		Nonce:             statedb.GetNonce(call.From),
		Value:             call.Value,
		GasLimit:          call.Gas,
		GasPrice:          call.GasPrice,
		GasFeeCap:         call.GasFeeCap,
		GasTipCap:         call.GasTipCap,
		Data:              call.Data,
		AccessList:        call.AccessList,
		SkipAccountChecks: false,
	}
}

// CallContract executes a contract call, without committing changes to the state
func (e *EVMEmulator) CallContract(call ethereum.CallMsg, gasBurnEnable func(bool)) (*core.ExecutionResult, error) {
	// Ensure message is initialized properly.
	if call.Gas == 0 {
		call.Gas = e.gasLimits.Call
	}
	if call.Value == nil {
		call.Value = big.NewInt(0)
	}

	pendingHeader := e.BlockchainDB().GetPendingHeader(e.timestamp)

	// run the EVM code on a buffered state (so that writes are not committed)
	statedb := e.StateDB().Buffered().StateDB()

	return e.applyMessage(coreMsgFromCallMsg(call, statedb), statedb, pendingHeader, gasBurnEnable, nil)
}

func (e *EVMEmulator) applyMessage(
	msg *core.Message,
	statedb vm.StateDB,
	header *types.Header,
	gasBurnEnable func(bool),
	tracer tracers.Tracer,
) (res *core.ExecutionResult, err error) {
	// Set msg gas price to 0
	msg.GasPrice = big.NewInt(0)
	msg.GasFeeCap = big.NewInt(0)
	msg.GasTipCap = big.NewInt(0)

	blockContext := core.NewEVMBlockContext(header, e.ChainContext(), nil)
	txContext := core.NewEVMTxContext(msg)

	vmConfig := e.vmConfig
	vmConfig.Tracer = tracer

	vmEnv := vm.NewEVM(blockContext, txContext, statedb, e.chainConfig, vmConfig)

	if msg.GasLimit > e.gasLimits.Call {
		msg.GasLimit = e.gasLimits.Call
	}

	gasPool := core.GasPool(msg.GasLimit)
	vmEnv.Reset(txContext, statedb)
	if gasBurnEnable != nil {
		gasBurnEnable(true)
		defer gasBurnEnable(false)
	}

	caughtErr := panicutil.CatchAllExcept(
		func() {
			// catch any exceptions during the execution, so that an EVM receipt is produced
			res, err = core.ApplyMessage(vmEnv, msg, &gasPool)
		},
		vmexceptions.AllProtocolLimits...,
	)
	if caughtErr != nil {
		return nil, caughtErr
	}
	return res, err
}

func (e *EVMEmulator) SendTransaction(
	tx *types.Transaction,
	ctx isc.Sandbox,
	tracer tracers.Tracer,
) (receipt *types.Receipt, result *core.ExecutionResult, iscGasErr error, err error) {
	buf := e.StateDB().Buffered()
	statedb := buf.StateDB()
	pendingHeader := e.BlockchainDB().GetPendingHeader(e.timestamp)

	sender, err := types.Sender(e.Signer(), tx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid transaction: %w", err)
	}
	nonce := e.StateDB().GetNonce(sender)
	if tx.Nonce() != nonce {
		return nil, nil, nil, fmt.Errorf("invalid transaction nonce: got %d, want %d", tx.Nonce(), nonce)
	}

	msg, err := core.TransactionToMessage(tx, types.MakeSigner(e.chainConfig, pendingHeader.Number), pendingHeader.BaseFee)
	if err != nil {
		return nil, nil, nil, err
	}

	var gasBurnEnable func(bool)
	if ctx != nil {
		gasBurnEnable = ctx.Privileged().GasBurnEnable
	}
	result, err = e.applyMessage(
		msg,
		statedb,
		pendingHeader,
		gasBurnEnable,
		tracer,
	)

	gasUsed := uint64(0)
	if result != nil {
		gasUsed = result.UsedGas
		if ctx != nil {
			// convert burnt EVM gas to ISC gas
			chainInfo := ctx.ChainInfo()
			ctx.Privileged().GasBurnEnable(true)
			iscGasErr = panicutil.CatchPanic(
				func() {
					ctx.Gas().Burn(
						gas.BurnCodeEVM1P,
						gas.EVMGasToISC(result.UsedGas, &chainInfo.GasFeePolicy.EVMGasRatio),
					)
				},
			)
			ctx.Privileged().GasBurnEnable(false)
			if iscGasErr != nil {
				// out of gas when burning ISC gas, edit the EVM receipt so that it fails
				// receipt.Status = types.ReceiptStatusFailed
				result.Err = core.ErrInsufficientFunds
			}
			// amend the gas usage (to include any ISC gas from sandbox calls)
			gasUsed = gas.ISCGasBudgetToEVM(ctx.Gas().Burned(), &chainInfo.GasFeePolicy.EVMGasRatio)
		}
	}

	cumulativeGasUsed := gasUsed
	index := uint(0)
	latest := e.BlockchainDB().GetLatestPendingReceipt()
	if latest != nil {
		cumulativeGasUsed += latest.CumulativeGasUsed
		index = latest.TransactionIndex + 1
	}

	receipt = &types.Receipt{
		Type:              tx.Type(),
		CumulativeGasUsed: cumulativeGasUsed,
		TxHash:            tx.Hash(),
		GasUsed:           gasUsed,
		Logs:              statedb.GetLogs(tx.Hash()),
		BlockNumber:       pendingHeader.Number,
		TransactionIndex:  index,
	}
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	if result == nil || result.Failed() {
		receipt.Status = types.ReceiptStatusFailed
	} else {
		receipt.Status = types.ReceiptStatusSuccessful
	}

	if msg.To == nil {
		receipt.ContractAddress = crypto.CreateAddress(msg.From, tx.Nonce())
	}

	buf.Commit()
	e.BlockchainDB().AddTransaction(tx, receipt)

	return receipt, result, iscGasErr, err
}

func (e *EVMEmulator) MintBlock() {
	e.BlockchainDB().MintBlock(e.timestamp)
}

func (e *EVMEmulator) Signer() types.Signer {
	return evmutil.Signer(e.chainConfig.ChainID)
}

type chainContext struct {
	engine consensus.Engine
}

var _ core.ChainContext = &chainContext{}

func (c *chainContext) Engine() consensus.Engine {
	return c.engine
}

func (c *chainContext) GetHeader(common.Hash, uint64) *types.Header {
	panic("not implemented")
}
