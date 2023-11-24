package solo

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/tracers"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/chainutil"
	"github.com/iotaledger/wasp/packages/evm/jsonrpc"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/testutil"

	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/trie"
)

// jsonRPCSoloBackend is the implementation of [jsonrpc.ChainBackend] for Solo
// tests.
type jsonRPCSoloBackend struct {
	Chain     *Chain
	baseToken api.InfoResBaseToken
	snapshots []*Snapshot
}

func newJSONRPCSoloBackend(chain *Chain, baseToken api.InfoResBaseToken) jsonrpc.ChainBackend {
	return &jsonRPCSoloBackend{Chain: chain, baseToken: baseToken}
}

func (b *jsonRPCSoloBackend) EVMSendTransaction(tx *types.Transaction) error {
	_, err := b.Chain.PostEthereumTransaction(tx)
	return err
}

func (b *jsonRPCSoloBackend) EVMCall(chainOutputs *isc.ChainOutputs, callMsg ethereum.CallMsg) ([]byte, error) {
	return chainutil.EVMCall(b.Chain, chainOutputs, callMsg, b.baseToken.Decimals)
}

func (b *jsonRPCSoloBackend) EVMEstimateGas(chainOutputs *isc.ChainOutputs, callMsg ethereum.CallMsg) (uint64, error) {
	return chainutil.EVMEstimateGas(b.Chain, chainOutputs, callMsg, b.baseToken.Decimals)
}

func (b *jsonRPCSoloBackend) EVMTraceTransaction(
	chainOutputs *isc.ChainOutputs,
	timestamp time.Time,
	iscRequestsInBlock []isc.Request,
	txIndex uint64,
	tracer tracers.Tracer,
) error {
	return chainutil.EVMTraceTransaction(
		b.Chain,
		chainOutputs,
		timestamp,
		iscRequestsInBlock,
		txIndex,
		tracer,
	)
}

func (b *jsonRPCSoloBackend) ISCCallView(chainState state.State, scName, funName string, args dict.Dict) (dict.Dict, error) {
	return b.Chain.CallViewAtState(chainState, scName, funName, args)
}

func (b *jsonRPCSoloBackend) ISCLatestChainOutputs() (*isc.ChainOutputs, error) {
	latestAnchorOutput, err := b.Chain.LatestChainOutputs(chaintypes.ActiveOrCommittedState)
	if err != nil {
		return nil, fmt.Errorf("could not get latest AnchorOutput: %w", err)
	}
	return latestAnchorOutput, nil
}

func (b *jsonRPCSoloBackend) ISCLatestState() state.State {
	latestState, err := b.Chain.LatestState(chaintypes.ActiveOrCommittedState)
	if err != nil {
		panic(err)
	}
	return latestState
}

func (b *jsonRPCSoloBackend) ISCStateByBlockIndex(blockIndex uint32) (state.State, error) {
	return b.Chain.store.StateByIndex(blockIndex)
}

func (b *jsonRPCSoloBackend) ISCStateByTrieRoot(trieRoot trie.Hash) (state.State, error) {
	return b.Chain.store.StateByTrieRoot(trieRoot)
}

func (b *jsonRPCSoloBackend) BaseToken() api.InfoResBaseToken {
	return b.baseToken
}

func (b *jsonRPCSoloBackend) ISCChainID() *isc.ChainID {
	return &b.Chain.ChainID
}

func (b *jsonRPCSoloBackend) RevertToSnapshot(i int) error {
	if i < 0 || i >= len(b.snapshots) {
		return errors.New("invalid snapshot index")
	}
	b.Chain.Env.RestoreSnapshot(b.snapshots[i])
	b.snapshots = b.snapshots[:i]
	return nil
}

func (b *jsonRPCSoloBackend) TakeSnapshot() (int, error) {
	b.snapshots = append(b.snapshots, b.Chain.Env.TakeSnapshot())
	return len(b.snapshots) - 1, nil
}

func (b *jsonRPCSoloBackend) BaseTokenDecimals() uint32 {
	return b.baseToken.Decimals
}

/*
indexDbEngine hivedb.Engine,
indexDbPath string,
db, err := database.DatabaseWithDefaultSettings(indexDbPath, true, indexDbEngine, false)

	if err != nil {
		panic(err)
	}

path.Join(indexDbPath, backend.ISCChainID().String())
*/
func (ch *Chain) EVM() *jsonrpc.EVMChain {
	return jsonrpc.NewEVMChain(
		newJSONRPCSoloBackend(ch, *testutil.TokenInfo),
		ch.Env.publisher,
		true,
		ch.Env.getDB(dbKindEVMJSONRPCIndex, ch.ChainID),
		ch.log,
	)
}

func (ch *Chain) PostEthereumTransaction(tx *types.Transaction) (dict.Dict, error) {
	req, err := isc.NewEVMOffLedgerTxRequest(ch.ChainID, tx)
	if err != nil {
		return nil, err
	}
	return ch.RunOffLedgerRequest(req)
}

var EthereumAccounts [10]*ecdsa.PrivateKey

func init() {
	for i := 0; i < len(EthereumAccounts); i++ {
		seed := crypto.Keccak256([]byte(fmt.Sprintf("seed %d", i)))
		key, err := crypto.ToECDSA(seed)
		if err != nil {
			panic(err)
		}
		EthereumAccounts[i] = key
	}
}

func (ch *Chain) EthereumAccountByIndexWithL2Funds(i int, baseTokens ...iotago.BaseToken) (*ecdsa.PrivateKey, common.Address) {
	key := EthereumAccounts[i]
	addr := crypto.PubkeyToAddress(key.PublicKey)
	ch.GetL2FundsFromFaucet(isc.NewEthereumAddressAgentID(ch.ChainID, addr), baseTokens...)
	return key, addr
}

func NewEthereumAccount() (*ecdsa.PrivateKey, common.Address) {
	key, err := crypto.GenerateKey()
	if err != nil {
		panic(err)
	}
	return key, crypto.PubkeyToAddress(key.PublicKey)
}

func (ch *Chain) NewEthereumAccountWithL2Funds(baseTokens ...iotago.BaseToken) (*ecdsa.PrivateKey, common.Address) {
	key, addr := NewEthereumAccount()
	ch.GetL2FundsFromFaucet(isc.NewEthereumAddressAgentID(ch.ChainID, addr), baseTokens...)
	return key, addr
}
