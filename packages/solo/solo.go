// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package solo

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"math/big"
	"math/rand"
	"slices"
	"sync"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/log"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/evm/evmlogger"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/metrics"
	"github.com/iotaledger/wasp/packages/origin"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/publisher"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/state/indexedstore"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
	"github.com/iotaledger/wasp/packages/testutil/utxodb"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/coreprocessors"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/migrations"
	"github.com/iotaledger/wasp/packages/vm/processors"
	_ "github.com/iotaledger/wasp/packages/vm/sandbox"
)

const (
	MaxRequestsInBlock = 100
)

// Solo is a structure which contains global parameters of the test: one per test instance
type Solo struct {
	// instance of the test
	T                               Context
	logger                          log.Logger
	db                              kvstore.KVStore
	utxoDB                          *utxodb.UtxoDB
	chainsMutex                     sync.RWMutex
	ledgerMutex                     sync.RWMutex
	chains                          map[isc.ChainID]*Chain
	processorConfig                 *processors.Config
	disableAutoAdjustStorageDeposit bool
	enableGasBurnLogging            bool
	seed                            cryptolib.Seed
	publisher                       *publisher.Publisher
	ctx                             context.Context
}

// data to be persisted in the snapshot
type chainData struct {
	// Name is the name of the chain
	Name string

	// StateControllerKeyPair signature scheme of the chain address, the one used to control funds owned by the chain.
	// In Solo it is Ed25519 signature scheme (in full Wasp environment is is a BLS address)
	StateControllerKeyPair *cryptolib.KeyPair

	// ChainID is the ID of the chain (in this version alias of the ChainAddress)
	ChainID isc.ChainID

	// OriginatorPrivateKey the key pair used to create the chain (origin transaction).
	// It is a default key pair in many of Solo calls which require private key.
	OriginatorPrivateKey *cryptolib.KeyPair

	// ValidatorFeeTarget is the agent ID to which all fees are accrued. By default, it is equal to OriginatorAgentID
	ValidatorFeeTarget isc.AgentID

	db         kvstore.KVStore
	writeMutex *sync.Mutex

	migrationScheme *migrations.MigrationScheme
}

type dbKind byte

const (
	dbKindChainState = dbKind(iota)
	dbKindEVMJSONRPCIndex
)

// Chain represents state of individual chain.
// There may be several parallel instances of the chain in the 'solo' test
type Chain struct {
	chainData

	StateControllerAddress iotago.Address
	OriginatorAddress      iotago.Address
	OriginatorAgentID      isc.AgentID

	// Env is a pointer to the global structure of the 'solo' test
	Env *Solo

	// Store is where the chain data (blocks, state) is stored
	store indexedstore.IndexedStore
	// Log is the named logger of the chain
	log log.Logger
	// global processor cache
	proc *processors.Cache
	// related to asynchronous backlog processing
	runVMMutex sync.Mutex
	// mempool of the chain is used in Solo to mimic a real node
	mempool Mempool

	RequestsBlock uint32

	metrics *metrics.ChainMetrics

	migrationScheme *migrations.MigrationScheme

	FakeChainNodes    func() []peering.PeerStatusProvider
	FakeCommitteeInfo func() *chaintypes.CommitteeInfo
}

type InitOptions struct {
	AutoAdjustStorageDeposit bool
	Debug                    bool
	GasBurnLogEnabled        bool
	Seed                     cryptolib.Seed
	ExtraVMTypes             map[string]processors.VMConstructor
	Log                      log.Logger
}

func DefaultInitOptions() *InitOptions {
	return &InitOptions{
		Debug:                    false,
		Seed:                     cryptolib.Seed{},
		AutoAdjustStorageDeposit: false, // is OFF by default
		GasBurnLogEnabled:        true,  // is ON by default
	}
}

// New creates an instance of the Solo environment
// If solo is used for unit testing, 't' should be the *testing.T instance;
// otherwise it can be either nil or an instance created with NewTestContext.
func New(t Context, initOptions ...*InitOptions) *Solo {
	opt := DefaultInitOptions()
	if len(initOptions) > 0 {
		opt = initOptions[0]
	}
	if opt.Log == nil {
		opt.Log = testlogger.NewSimple(opt.Debug, log.WithName(t.Name()))
	}
	evmlogger.Init(opt.Log)

	ctx, cancelCtx := context.WithCancel(context.Background())
	t.Cleanup(cancelCtx)
	ret := &Solo{
		T:                               t,
		logger:                          opt.Log,
		db:                              mapdb.NewMapDB(),
		utxoDB:                          utxodb.New(testutil.L1API),
		chains:                          make(map[isc.ChainID]*Chain),
		processorConfig:                 coreprocessors.NewConfigWithCoreContracts(),
		disableAutoAdjustStorageDeposit: !opt.AutoAdjustStorageDeposit,
		enableGasBurnLogging:            opt.GasBurnLogEnabled,
		seed:                            opt.Seed,
		publisher:                       publisher.New(opt.Log.NewChildLogger("publisher")),
		ctx:                             ctx,
	}
	ret.logger.LogInfof("Solo environment has been created")

	for vmType, constructor := range opt.ExtraVMTypes {
		err := ret.processorConfig.RegisterVMType(vmType, constructor)
		require.NoError(t, err)
	}

	_ = ret.publisher.Events.Published.Hook(func(ev *publisher.ISCEvent[any]) {
		hrp := testutil.L1API.ProtocolParameters().Bech32HRP()
		ret.logger.LogInfof("solo publisher: %s", ev.String(hrp))
	})

	go ret.publisher.Run(ctx)

	go ret.batchLoop()

	return ret
}

func (env *Solo) Log() log.Logger {
	return env.logger
}

func (env *Solo) batchLoop() {
	for {
		time.Sleep(50 * time.Millisecond)
		chains := func() []*Chain {
			env.chainsMutex.Lock()
			defer env.chainsMutex.Unlock()
			return lo.Values(env.chains)
		}()
		for _, ch := range chains {
			ch.collateAndRunBatch()
		}
	}
}

func (env *Solo) IterateChainTrieDBs(
	f func(chainID *isc.ChainID, k []byte, v []byte),
) {
	env.chainsMutex.Lock()
	defer env.chainsMutex.Unlock()

	chainIDs := lo.Keys(env.chains)
	slices.SortFunc(chainIDs, func(a, b isc.ChainID) int { return bytes.Compare(a.Bytes(), b.Bytes()) })
	for _, chID := range chainIDs {
		chID := chID // prevent loop variable aliasing
		ch := env.chains[chID]
		lo.Must0(ch.db.Iterate(nil, func(k []byte, v []byte) bool {
			f(&chID, k, v)
			return true
		}))
	}
}

func (env *Solo) IterateChainLatestStates(
	prefix kv.Key,
	f func(chainID *isc.ChainID, k []byte, v []byte),
) {
	env.chainsMutex.Lock()
	defer env.chainsMutex.Unlock()

	chainIDs := lo.Keys(env.chains)
	slices.SortFunc(chainIDs, func(a, b isc.ChainID) int { return bytes.Compare(a.Bytes(), b.Bytes()) })
	for _, chID := range chainIDs {
		chID := chID // prevent loop variable aliasing
		ch := env.chains[chID]
		store := indexedstore.New(state.NewStoreWithUniqueWriteMutex(ch.db))
		state, err := store.LatestState()
		require.NoError(env.T, err)
		state.IterateSorted(prefix, func(k kv.Key, v []byte) bool {
			f(&chID, []byte(k), v)
			return true
		})
	}
}

func (env *Solo) Publisher() *publisher.Publisher {
	return env.publisher
}

func (env *Solo) GetChains() map[isc.ChainID]*Chain {
	env.chainsMutex.Lock()
	defer env.chainsMutex.Unlock()
	return maps.Clone(env.chains)
}

func (env *Solo) GetChainByName(name string) *Chain {
	env.chainsMutex.Lock()
	defer env.chainsMutex.Unlock()
	for _, ch := range env.chains {
		if ch.Name == name {
			return ch
		}
	}
	panic("chain not found")
}

// WithNativeContract registers a native contract so that it may be deployed
func (env *Solo) WithNativeContract(c *coreutil.ContractProcessor) *Solo {
	env.processorConfig.RegisterNativeContract(c)
	return env
}

// NewChain deploys new default chain instance.
func (env *Solo) NewChain(depositFundsForOriginator ...bool) *Chain {
	ret, _ := env.NewChainExt(nil, 0, 0, "chain1")
	if len(depositFundsForOriginator) == 0 || depositFundsForOriginator[0] {
		// deposit some tokens for the chain originator
		err := ret.DepositAssetsToL2(isc.NewAssetsBaseTokens(5*isc.Million), nil)
		require.NoError(env.T, err)
	}
	return ret
}

func (env *Solo) deployChain(
	chainOriginator *cryptolib.KeyPair,
	initBaseTokens iotago.BaseToken,
	initMana iotago.Mana,
	name string,
	originParams ...dict.Dict,
) (chainData, *iotago.SignedTransaction) {
	env.logger.LogDebugf("deploying new chain '%s'", name)

	if chainOriginator == nil {
		chainOriginator = env.NewKeyPairFromIndex(-1000 + len(env.chains)) // making new originator for each new chain
		originatorAddr := chainOriginator.GetPublicKey().AsEd25519Address()
		_, err := env.utxoDB.GetFundsFromFaucet(originatorAddr)
		require.NoError(env.T, err)
	}

	initParams := dict.Dict{
		origin.ParamChainOwner: isc.NewAgentID(chainOriginator.Address()).Bytes(),
		// FIXME this will cause import cycle
		// origin.ParamWaspVersion: codec.String.Encode(app.Version),
	}
	if len(originParams) > 0 {
		for k, v := range originParams[0] {
			initParams[k] = v
		}
	}

	stateControllerKey := env.NewKeyPairFromIndex(-1) // leaving positive indices to user
	stateControllerAddr := stateControllerKey.GetPublicKey().AsEd25519Address()

	originatorAddr := chainOriginator.GetPublicKey().AsEd25519Address()
	originatorAgentID := isc.NewAgentID(originatorAddr)

	initialL1Balance := env.L1BaseTokens(originatorAddr)

	outs := env.utxoDB.GetUnspentOutputs(originatorAddr)
	originTx, chainOutputs, chainID, err := origin.NewChainOriginTransaction(
		chainOriginator,
		stateControllerAddr,
		stateControllerAddr,
		initBaseTokens, // will be adjusted to min storage deposit + DefaultMinBaseTokensOnCommonAccount
		initMana,
		initParams,
		outs,
		env.SlotIndex(),
		0, // TODO could be the latest instead ? :thinking:
		testutil.L1APIProvider,
		testutil.TokenInfo,
	)
	require.NoError(env.T, err)

	anchor, _, err := transaction.GetAnchorFromTransaction(originTx.Transaction)
	require.NoError(env.T, err)

	err = env.AddToLedger(originTx)
	require.NoError(env.T, err)
	env.AssertL1BaseTokens(originatorAddr, initialL1Balance-anchor.Deposit)

	env.logger.LogInfof("deploying new chain '%s'. ID: %s, state controller address: %s",
		name, chainID.Bech32(testutil.L1API.ProtocolParameters().Bech32HRP()), stateControllerAddr.Bech32(testutil.L1API.ProtocolParameters().Bech32HRP()))
	env.logger.LogInfof("     chain '%s'. state controller address: %s", chainID.Bech32(testutil.L1API.ProtocolParameters().Bech32HRP()), stateControllerAddr.Bech32(testutil.L1API.ProtocolParameters().Bech32HRP()))
	env.logger.LogInfof("     chain '%s'. originator address: %s", chainID.Bech32(testutil.L1API.ProtocolParameters().Bech32HRP()), originatorAddr.Bech32(testutil.L1API.ProtocolParameters().Bech32HRP()))

	chainDB := env.getDB(dbKindChainState, chainID)
	require.NoError(env.T, err)
	store := indexedstore.New(state.NewStoreWithUniqueWriteMutex(chainDB))
	_, err = origin.InitChainByAnchorOutput(store, chainOutputs, env.L1APIProvider(), testutil.TokenInfo)
	require.NoError(env.T, err)

	{
		block, err2 := store.LatestBlock()
		require.NoError(env.T, err2)
		env.logger.LogInfof("     chain '%s'. origin trie root: %s", chainID.ShortString(), block.TrieRoot())
	}

	return chainData{
		Name:                   name,
		ChainID:                chainID,
		StateControllerKeyPair: stateControllerKey,
		OriginatorPrivateKey:   chainOriginator,
		ValidatorFeeTarget:     originatorAgentID,
		db:                     chainDB,
	}, originTx
}

func (env *Solo) getDB(kind dbKind, chainID isc.ChainID) kvstore.KVStore {
	return lo.Must(env.db.WithRealm(append([]byte{byte(kind)}, chainID[:]...)))
}

// NewChainExt returns also origin and init transactions. Used for core testing
//
// If 'chainOriginator' is nil, new one is generated and utxodb.FundsFromFaucetAmount (many) base tokens are loaded from the UTXODB faucet.
// ValidatorFeeTarget will be set to OriginatorAgentID, and can be changed after initialization.
// To deploy a chain instance the following steps are performed:
//   - chain signature scheme (private key), chain address and chain ID are created
//   - empty virtual state is initialized
//   - origin transaction is created by the originator and added to the UTXODB
//   - 'init' request transaction to the 'root' contract is created and added to UTXODB
//   - backlog processing threads (goroutines) are started
//   - VM processor cache is initialized
//   - 'init' request is run by the VM. The 'root' contracts deploys the rest of the core contracts:
//
// Upon return, the chain is fully functional to process requests
func (env *Solo) NewChainExt(
	chainOriginator *cryptolib.KeyPair,
	initBaseTokens iotago.BaseToken,
	initMana iotago.Mana,
	name string,
	originParams ...dict.Dict,
) (*Chain, *iotago.SignedTransaction) {
	chData, originTx := env.deployChain(
		chainOriginator,
		initBaseTokens,
		initMana,
		name,
		originParams...,
	)

	env.chainsMutex.Lock()
	defer env.chainsMutex.Unlock()
	ch := env.addChain(chData)

	ch.log.LogInfof("chain '%s' deployed. Chain ID: %s", ch.Name, ch.ChainID.Bech32(testutil.L1API.ProtocolParameters().Bech32HRP()))
	return ch, originTx
}

func (env *Solo) addChain(chData chainData) *Chain {
	ch := &Chain{
		chainData:              chData,
		StateControllerAddress: chData.StateControllerKeyPair.GetPublicKey().AsEd25519Address(),
		OriginatorAddress:      chData.OriginatorPrivateKey.GetPublicKey().AsEd25519Address(),
		OriginatorAgentID:      isc.NewAgentID(chData.OriginatorPrivateKey.GetPublicKey().AsEd25519Address()),
		Env:                    env,
		store:                  indexedstore.New(state.NewStoreWithUniqueWriteMutex(chData.db)),
		proc:                   processors.MustNew(env.processorConfig),
		log:                    env.logger.NewChildLogger(chData.Name),
		metrics:                metrics.NewChainMetricsProvider().GetChainMetrics(chData.ChainID),
		mempool:                newMempool(env.Timestamp, chData.ChainID),
		migrationScheme:        chData.migrationScheme,
	}
	env.chains[chData.ChainID] = ch
	return ch
}

// AddToLedger adds (synchronously confirms) transaction to the UTXODB ledger. Return error if it is
// invalid or double spend
func (env *Solo) AddToLedger(tx *iotago.SignedTransaction) error {
	env.logger.LogDebugf("adding tx to L1 (ID: %s) %s",
		lo.Must(tx.Transaction.ID()).ToHex(),
		string(lo.Must(testutil.L1API.JSONEncode(tx))),
	)
	return env.utxoDB.AddToLedger(tx)
}

// RequestsForChain parses the transaction and returns all requests contained in it which have chainID as the target
func (env *Solo) RequestsForChain(tx *iotago.Transaction, chainID isc.ChainID) ([]isc.Request, error) {
	env.chainsMutex.RLock()
	defer env.chainsMutex.RUnlock()

	m := env.requestsByChain(tx)
	ret, ok := m[chainID]
	if !ok {
		return nil, fmt.Errorf("chain %s does not exist", chainID.Bech32(testutil.L1API.ProtocolParameters().Bech32HRP()))
	}
	return ret, nil
}

// requestsByChain parses the transaction and extracts those outputs which are interpreted as a request to a chain
func (env *Solo) requestsByChain(tx *iotago.Transaction) map[isc.ChainID][]isc.Request {
	ret, err := isc.RequestsInTransaction(tx)
	require.NoError(env.T, err)
	return ret
}

// AddRequestsToMempool adds all the requests to the chain mempool,
func (env *Solo) AddRequestsToMempool(ch *Chain, reqs []isc.Request) {
	ch.mempool.ReceiveRequests(reqs...)
}

// EnqueueRequests adds requests contained in the transaction to mempools of respective target chains
func (env *Solo) EnqueueRequests(tx *iotago.SignedTransaction) {
	env.chainsMutex.RLock()
	defer env.chainsMutex.RUnlock()

	requests := env.requestsByChain(tx.Transaction)

	for chainID, reqs := range requests {
		ch, ok := env.chains[chainID]
		if !ok {
			env.logger.LogInfof("dispatching requests. Unknown chain: %s", chainID.Bech32(testutil.L1API.ProtocolParameters().Bech32HRP()))
			continue
		}
		if len(reqs) > 0 {
			env.logger.LogInfof("dispatching %d requests to chain %s", len(reqs), chainID)
			ch.mempool.ReceiveRequests(reqs...)
		}
	}
}

func (ch *Chain) GetChainOutputsFromL1() *isc.ChainOutputs {
	anchor := ch.Env.utxoDB.GetAnchorOutputs(ch.ChainID.AsAddress())
	require.EqualValues(ch.Env.T, 1, len(anchor))
	account := ch.Env.utxoDB.GetAccountOutputs(ch.ChainID.AsAddress())
	require.LessOrEqual(ch.Env.T, len(account), 1)
	for anchorOutputID, anchorOutput := range anchor {
		for accountOutputID, accountOutput := range account {
			return isc.NewChainOutputs(
				anchorOutput,
				anchorOutputID,
				accountOutput,
				accountOutputID,
			)
		}
		// state index 0 => no account output
		require.EqualValues(ch.Env.T, 0, anchorOutput.StateIndex)
		return isc.NewChainOutputs(
			anchorOutput,
			anchorOutputID,
			nil,
			iotago.OutputID{},
		)
	}
	panic("unreachable")
}

// collateBatch selects requests which are not time locked
// returns batch and and 'remains unprocessed' flag
func (ch *Chain) collateBatch() []isc.Request {
	// emulating variable sized blocks
	maxBatch := MaxRequestsInBlock - rand.Intn(MaxRequestsInBlock/3)
	requests := ch.mempool.RequestBatchProposal()
	batchSize := len(requests)

	if batchSize > maxBatch {
		batchSize = maxBatch
	}

	return requests[:batchSize]
}

func (ch *Chain) collateAndRunBatch() {
	ch.runVMMutex.Lock()
	defer ch.runVMMutex.Unlock()
	if ch.Env.ctx.Err() != nil {
		return
	}
	batch := ch.collateBatch()
	if len(batch) > 0 {
		results := ch.runRequestsNolock(batch, "batchLoop")
		for _, res := range results {
			if res.Receipt.Error != nil {
				ch.log.LogErrorf("runRequestsSync: %v", res.Receipt.Error)
			}
		}
	}
}

func (ch *Chain) AddMigration(m migrations.Migration) {
	ch.migrationScheme.Migrations = append(ch.migrationScheme.Migrations, m)
}

func (ch *Chain) GetCandidateNodes() []*governance.AccessNodeInfo {
	return nil
}

func (ch *Chain) GetChainNodes() []peering.PeerStatusProvider {
	return ch.FakeChainNodes()
}

func (ch *Chain) GetCommitteeInfo() *chaintypes.CommitteeInfo {
	return ch.FakeCommitteeInfo()
}

func (ch *Chain) ID() isc.ChainID {
	return ch.ChainID
}

func (ch *Chain) Log() log.Logger {
	return ch.log
}

func (ch *Chain) Processors() *processors.Cache {
	return ch.proc
}

// ---------------------------------------------

func (env *Solo) UnspentOutputs(addr iotago.Address) iotago.OutputSet {
	return env.utxoDB.GetUnspentOutputs(addr)
}

func (env *Solo) L1NFTs(addr iotago.Address) map[iotago.OutputID]*iotago.NFTOutput {
	return env.utxoDB.GetAddressNFTs(addr)
}

// L1NativeTokens returns number of native tokens contained in the given address on the UTXODB ledger
func (env *Solo) L1NativeTokens(addr iotago.Address, nativeTokenID iotago.NativeTokenID) *big.Int {
	assets := env.L1Assets(addr)
	return assets.NativeTokens.ValueOrBigInt0(nativeTokenID)
}

func (env *Solo) L1BaseTokens(addr iotago.Address) iotago.BaseToken {
	return env.utxoDB.GetAddressBalanceBaseTokens(addr)
}

// L1Assets returns all ftokens of the address contained in the UTXODB ledger
func (env *Solo) L1Assets(addr iotago.Address) *isc.Assets {
	a := isc.NewAssetsBaseTokens(env.utxoDB.GetAddressBalanceBaseTokens(addr))
	for id, nt := range env.utxoDB.GetAddressBalanceNativeTokens(addr) {
		a.AddNativeTokens(id, nt)
	}
	a.AddNFTs(env.utxoDB.GetAddressBalanceNFTs(addr)...)
	return a
}

func (env *Solo) L1Ledger() *utxodb.UtxoDB {
	return env.utxoDB
}

type NFTMintedInfo struct {
	Output   iotago.Output
	OutputID iotago.OutputID
	NFTID    iotago.NFTID
}

// MintNFTL1 mints a single NFT with the `issuer` account and sends it to a `target` account.
// Base tokens in the NFT output are sent to the minimum storage deposit and are taken from the issuer account.
func (env *Solo) MintNFTL1(issuer *cryptolib.KeyPair, target iotago.Address, immutableMetadata iotago.MetadataFeatureEntries) (*isc.NFT, *NFTMintedInfo, error) {
	nfts, infos, err := env.MintNFTsL1(issuer, target, nil, []iotago.MetadataFeatureEntries{immutableMetadata})
	if err != nil {
		return nil, nil, err
	}
	return nfts[0], infos[0], nil
}

// MintNFTsL1 mints len(immutableMetadata) NFTs with the `issuer` account and sends them
// to a `target` account.
//
// If collectionOutputID is not nil, it must be an outputID of an NFTOutput owned by the issuer.
// All minted NFTs will belong to the given collection.
// See: https://github.com/iotaledger/tips/blob/main/tips/TIP-0027/tip-0027.md
//
// Base tokens in the NFT outputs are sent to the minimum storage deposit and are taken from the issuer account.
func (env *Solo) MintNFTsL1(issuer *cryptolib.KeyPair, target iotago.Address, collectionOutputID *iotago.OutputID, immutableMetadata []iotago.MetadataFeatureEntries) ([]*isc.NFT, []*NFTMintedInfo, error) {
	allOuts := env.utxoDB.GetUnspentOutputs(issuer.Address())

	tx, err := transaction.NewMintNFTsTransaction(
		issuer,
		collectionOutputID,
		target,
		immutableMetadata,
		allOuts,
		env.SlotIndex(),
		testutil.L1APIProvider,
	)
	if err != nil {
		return nil, nil, err
	}
	err = env.AddToLedger(tx)
	if err != nil {
		return nil, nil, err
	}

	outSet, err := tx.Transaction.OutputsSet()
	if err != nil {
		return nil, nil, err
	}

	var nfts []*isc.NFT
	var infos []*NFTMintedInfo
	for id, out := range outSet {
		if out, ok := out.(*iotago.NFTOutput); ok { //nolint:gocritic // false positive
			nftID := util.NFTIDFromNFTOutput(out, id)
			info := &NFTMintedInfo{
				OutputID: id,
				Output:   out,
				NFTID:    nftID,
			}
			nft := &isc.NFT{
				ID:       info.NFTID,
				Issuer:   out.ImmutableFeatureSet().Issuer().Address,
				Metadata: out.ImmutableFeatureSet().Metadata().Entries,
			}
			nfts = append(nfts, nft)
			infos = append(infos, info)
		}
	}
	return nfts, infos, nil
}

// SendL1 sends base or native tokens to another L1 address
func (env *Solo) SendL1(targetAddress iotago.Address, assets *isc.Assets, wallet *cryptolib.KeyPair) {
	allOuts := env.utxoDB.GetUnspentOutputs(wallet.Address())
	tx, err := transaction.NewTransferTransaction(
		&assets.FungibleTokens,
		0,
		wallet.Address(),
		wallet,
		targetAddress,
		allOuts,
		nil,
		env.SlotIndex(),
		env.disableAutoAdjustStorageDeposit,
		env.L1APIProvider(),
	)
	require.NoError(env.T, err)
	err = env.AddToLedger(tx)
	require.NoError(env.T, err)
}

func (env *Solo) L1APIProvider() iotago.APIProvider {
	return iotago.SingleVersionProvider(testutil.L1API)
}

func (env *Solo) TokenInfo() *api.InfoResBaseToken {
	return testutil.TokenInfo
}
