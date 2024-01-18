// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package solo

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/evm/evmutil"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/metrics"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/state/indexedstore"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/util/rwutil"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/blob"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	vmerrors "github.com/iotaledger/wasp/packages/vm/core/errors"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/packages/vm/gas"
	"github.com/iotaledger/wasp/packages/vm/vmtypes"
)

// solo chain implements Chain interface
var _ chaintypes.Chain = &Chain{}

// String is string representation for main parameters of the chain
func (ch *Chain) String() string {
	w := new(rwutil.Buffer)
	fmt.Fprintf(w, "Chain ID: %s\n", ch.ChainID)
	fmt.Fprintf(w, "Chain state controller: %s\n", ch.StateControllerAddress)
	block, err := ch.store.LatestBlock()
	require.NoError(ch.Env.T, err)
	fmt.Fprintf(w, "Root commitment: %s\n", block.TrieRoot())
	fmt.Fprintf(w, "UTXODB genesis address: %s\n", ch.Env.utxoDB.GenesisAddress())
	return string(*w)
}

// DumpAccounts dumps all account balances into the human-readable string
func (ch *Chain) DumpAccounts() string {
	_, chainOwnerID, _ := ch.GetInfo()
	ret := fmt.Sprintf("ChainID: %s\nChain owner: %s\n",
		ch.ChainID.Bech32(ch.Env.L1APIProvider().CommittedAPI().ProtocolParameters().Bech32HRP()),
		chainOwnerID.Bech32(ch.Env.L1APIProvider().CommittedAPI().ProtocolParameters().Bech32HRP()),
	)
	acc := ch.L2Accounts()
	for i := range acc {
		aid := acc[i]
		ret += fmt.Sprintf("  %s:\n", aid.Bech32(ch.Env.L1APIProvider().CommittedAPI().ProtocolParameters().Bech32HRP()))
		bals := ch.L2Assets(aid)
		ret += fmt.Sprintf("%s\n", bals.String())
	}
	return ret
}

// FindContract is a view call to the 'root' smart contract on the chain.
// It returns blobCache record of the deployed smart contract with the given name
func (ch *Chain) FindContract(scName string) (*root.ContractRecord, error) {
	retDict, err := ch.CallView(root.ViewFindContract.Message(isc.Hn(scName)))
	if err != nil {
		return nil, err
	}
	ok, err := root.ViewFindContract.Output1.Decode(retDict)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("smart contract '%s' not found", scName)
	}
	return root.ViewFindContract.Output2.Decode(retDict)
}

// GetBlobInfo return info about blob with the given hash with existence flag
// The blob information is returned as a map of pairs 'blobFieldName': 'fieldDataLength'
func (ch *Chain) GetBlobInfo(blobHash hashing.HashValue) (map[string]uint32, bool) {
	res, err := ch.CallView(blob.ViewGetBlobInfo.Message(blobHash))
	require.NoError(ch.Env.T, err)
	sizes, err := blob.ViewGetBlobInfo.Output.Decode(res)
	require.NoError(ch.Env.T, err)
	return sizes, len(sizes) > 0
}

func (ch *Chain) GetGasFeePolicy() *gas.FeePolicy {
	res, err := ch.CallView(governance.ViewGetFeePolicy.Message())
	require.NoError(ch.Env.T, err)
	return lo.Must(governance.ViewGetFeePolicy.Output.Decode(res))
}

func (ch *Chain) SetGasFeePolicy(user *cryptolib.KeyPair, fp *gas.FeePolicy) {
	_, err := ch.PostRequestOffLedger(NewCallParams(governance.FuncSetFeePolicy.Message(fp)), user)
	require.NoError(ch.Env.T, err)
}

func (ch *Chain) GetGasLimits() *gas.Limits {
	res, err := ch.CallView(governance.ViewGetGasLimits.Message())
	require.NoError(ch.Env.T, err)
	return lo.Must(governance.ViewGetGasLimits.Output.Decode(res))
}

func (ch *Chain) SetGasLimits(user *cryptolib.KeyPair, gl *gas.Limits) {
	_, err := ch.PostRequestOffLedger(NewCallParams(governance.FuncSetGasLimits.Message(gl)), user)
	require.NoError(ch.Env.T, err)
}

// UploadBlob calls core 'blob' smart contract blob.FuncStoreBlob entry point to upload blob
// data to the chain. It returns hash of the blob, the unique identifier of it.
// The parameters must be either a dict.Dict, or a sequence of pairs 'fieldName': 'fieldValue'
// Requires at least 2 x gasFeeEstimate to be on sender's L2 account
func (ch *Chain) UploadBlob(user *cryptolib.KeyPair, fields dict.Dict) (ret hashing.HashValue, err error) {
	if user == nil {
		user = ch.OriginatorPrivateKey
	}

	expectedHash := blob.GetBlobHash(fields)
	if _, ok := ch.GetBlobInfo(expectedHash); ok {
		// blob exists, return hash of existing
		return expectedHash, nil
	}
	req := NewCallParams(blob.FuncStoreBlob.Message(fields))
	_, estimate, err := ch.EstimateGasOffLedger(req, nil, true)
	if err != nil {
		return [32]byte{}, err
	}
	req.WithGasBudget(estimate.GasBurned)
	res, err := ch.PostRequestOffLedger(req, user)
	if err != nil {
		return ret, err
	}
	resBin := res.Get(blob.ParamHash)
	if resBin == nil {
		err = errors.New("internal error: no hash returned")
		return ret, err
	}
	ret, err = codec.HashValue.Decode(resBin)
	if err != nil {
		return ret, err
	}
	require.EqualValues(ch.Env.T, expectedHash, ret)
	return ret, err
}

// UploadBlobFromFile uploads blob from file data in the specified blob field plus optional other fields
func (ch *Chain) UploadBlobFromFile(keyPair *cryptolib.KeyPair, fileName, fieldName string) (hashing.HashValue, error) {
	fileBinary, err := os.ReadFile(fileName)
	if err != nil {
		return hashing.HashValue{}, err
	}
	return ch.UploadBlob(keyPair, dict.Dict{kv.Key(fieldName): fileBinary})
}

// UploadWasm is a syntactic sugar of the UploadBlob used to upload Wasm binary to the chain.
//
//	parameter 'binaryCode' is the binary of Wasm smart contract program
//
// The blob for the Wasm binary used fixed field names which are statically known by the
// 'root' smart contract which is responsible for the deployment of contracts on the chain
func (ch *Chain) UploadWasm(keyPair *cryptolib.KeyPair, binaryCode []byte) (ret hashing.HashValue, err error) {
	return ch.UploadBlob(keyPair, dict.Dict{
		blob.VarFieldVMType:        codec.String.Encode(vmtypes.WasmTime),
		blob.VarFieldProgramBinary: binaryCode,
	})
}

// UploadWasmFromFile is a syntactic sugar to upload file content as blob data to the chain
func (ch *Chain) UploadWasmFromFile(keyPair *cryptolib.KeyPair, fileName string) (hashing.HashValue, error) {
	var binary []byte
	binary, err := os.ReadFile(fileName)
	if err != nil {
		return hashing.HashValue{}, err
	}
	return ch.UploadWasm(keyPair, binary)
}

// GetWasmBinary retrieves program binary in the format of Wasm blob from the chain by hash.
func (ch *Chain) GetWasmBinary(progHash hashing.HashValue) ([]byte, error) {
	res, err := ch.CallView(blob.ViewGetBlobField.Message(progHash, []byte(blob.VarFieldVMType)))
	if err != nil {
		return nil, err
	}
	vmType := string(lo.Must(blob.ViewGetBlobField.Output.Decode(res)))
	require.EqualValues(ch.Env.T, vmtypes.WasmTime, vmType)

	res, err = ch.CallView(blob.ViewGetBlobField.Message(progHash, []byte(blob.VarFieldProgramBinary)))
	if err != nil {
		return nil, err
	}
	return blob.ViewGetBlobField.Output.Decode(res)
}

// DeployContract deploys contract with the given name by its 'programHash'. 'sigScheme' represents
// the private key of the creator (nil defaults to chain originator). The 'creator' becomes an immutable
// property of the contract instance.
// The parameter 'programHash' can be one of the following:
//   - it is and ID of  the blob stored on the chain in the format of Wasm binary
//   - it can be a hash (ID) of the example smart contract ("hardcoded"). The "hardcoded"
//     smart contract must be made available with the call examples.AddProcessor
func (ch *Chain) DeployContract(user *cryptolib.KeyPair, name string, programHash hashing.HashValue, initParams ...dict.Dict) error {
	var d dict.Dict
	if len(initParams) > 0 {
		d = initParams[0]
	}
	_, err := ch.PostRequestSync(
		NewCallParams(root.FuncDeployContract.Message(name, programHash, d)).
			WithGasBudget(math.MaxUint64),
		user,
	)
	return err
}

// DeployWasmContract is syntactic sugar for uploading Wasm binary from file and
// deploying the smart contract in one call
func (ch *Chain) DeployWasmContract(keyPair *cryptolib.KeyPair, name, fname string, initParams ...dict.Dict) error {
	hprog, err := ch.UploadWasmFromFile(keyPair, fname)
	if err != nil {
		return err
	}
	return ch.DeployContract(keyPair, name, hprog, initParams...)
}

// DeployEVMContract deploys an evm contract on the chain
func (ch *Chain) DeployEVMContract(creator *ecdsa.PrivateKey, abiJSON string, bytecode []byte, value *big.Int, args ...interface{}) (common.Address, abi.ABI) {
	creatorAddress := crypto.PubkeyToAddress(creator.PublicKey)

	nonce := ch.Nonce(isc.NewEthereumAddressAgentID(ch.ChainID, creatorAddress))

	contractABI, err := abi.JSON(strings.NewReader(abiJSON))
	require.NoError(ch.Env.T, err)
	constructorArguments, err := contractABI.Pack("", args...)
	require.NoError(ch.Env.T, err)

	data := []byte{}
	data = append(data, bytecode...)
	data = append(data, constructorArguments...)

	gasLimit, err := ch.EVM().EstimateGas(ethereum.CallMsg{
		From:  creatorAddress,
		Value: value,
		Data:  data,
	}, nil)
	require.NoError(ch.Env.T, err)

	tx, err := types.SignTx(
		types.NewContractCreation(nonce, value, gasLimit, ch.EVM().GasPrice(), data),
		evmutil.Signer(big.NewInt(int64(ch.EVM().ChainID()))),
		creator,
	)
	require.NoError(ch.Env.T, err)

	err = ch.EVM().SendTransaction(tx)
	require.NoError(ch.Env.T, err)
	return crypto.CreateAddress(creatorAddress, nonce), contractABI
}

// GetInfo return main parameters of the chain:
//   - chainID
//   - agentID of the chain owner
//   - blobCache of contract deployed on the chain in the form of map 'contract hname': 'contract record'
func (ch *Chain) GetInfo() (isc.ChainID, isc.AgentID, map[isc.Hname]*root.ContractRecord) {
	res, err := ch.CallView(governance.ViewGetChainOwner.Message())
	require.NoError(ch.Env.T, err)

	chainOwnerID, err := governance.ViewGetChainOwner.Output.Decode(res)
	require.NoError(ch.Env.T, err)

	res, err = ch.CallView(root.ViewGetContractRecords.Message())
	require.NoError(ch.Env.T, err)

	contracts, err := root.ViewGetContractRecords.Output.Decode(res)
	require.NoError(ch.Env.T, err)
	return ch.ChainID, chainOwnerID, contracts
}

// GetEventsForContract calls the view in the 'blocklog' core smart contract to retrieve events for a given smart contract.
func (ch *Chain) GetEventsForContract(name string) ([]*isc.Event, error) {
	viewResult, err := ch.CallView(blocklog.ViewGetEventsForContract.Message(blocklog.EventsForContractQuery{
		Contract: isc.Hn(name),
	}))
	if err != nil {
		return nil, err
	}
	return blocklog.ViewGetEventsForContract.Output.Decode(viewResult)
}

// GetEventsForRequest calls the view in the 'blocklog' core smart contract to retrieve events for a given request.
func (ch *Chain) GetEventsForRequest(reqID isc.RequestID) ([]*isc.Event, error) {
	viewResult, err := ch.CallView(blocklog.ViewGetEventsForRequest.Message(reqID))
	if err != nil {
		return nil, err
	}
	return blocklog.ViewGetEventsForRequest.Output.Decode(viewResult)
}

// GetEventsForBlock calls the view in the 'blocklog' core smart contract to retrieve events for a given block.
func (ch *Chain) GetEventsForBlock(blockIndex uint32) ([]*isc.Event, error) {
	viewResult, err := ch.CallView(blocklog.ViewGetEventsForBlock.Message(&blockIndex))
	if err != nil {
		return nil, err
	}
	return blocklog.ViewGetEventsForBlock.Output2.Decode(viewResult)
}

// GetLatestBlockInfo return BlockInfo for the latest block in the chain
func (ch *Chain) GetLatestBlockInfo() *blocklog.BlockInfo {
	ret, err := ch.CallView(blocklog.ViewGetBlockInfo.Message(nil))
	require.NoError(ch.Env.T, err)
	return lo.Must(blocklog.ViewGetBlockInfo.Output2.Decode(ret))
}

func (ch *Chain) GetErrorMessageFormat(code isc.VMErrorCode) (string, error) {
	ret, err := ch.CallView(vmerrors.ViewGetErrorMessageFormat.Message(code))
	if err != nil {
		return "", err
	}
	return vmerrors.ViewGetErrorMessageFormat.Output.Decode(ret)
}

// GetBlockInfo return BlockInfo for the particular block index in the chain
func (ch *Chain) GetBlockInfo(blockIndex ...uint32) (*blocklog.BlockInfo, error) {
	ret, err := ch.CallView(blocklog.ViewGetBlockInfo.Message(coreutil.Optional(blockIndex...)))
	if err != nil {
		return nil, err
	}
	return blocklog.ViewGetBlockInfo.Output2.Decode(ret)
}

// IsRequestProcessed checks if the request is booked on the chain as processed
func (ch *Chain) IsRequestProcessed(reqID isc.RequestID) bool {
	ret, err := ch.CallView(blocklog.ViewIsRequestProcessed.Message(reqID))
	require.NoError(ch.Env.T, err)
	return lo.Must(blocklog.ViewIsRequestProcessed.Output.Decode(ret))
}

// GetRequestReceipt gets the log records for a particular request, the block index and request index in the block
func (ch *Chain) GetRequestReceipt(reqID isc.RequestID) (*blocklog.RequestReceipt, bool) {
	ret, err := ch.CallView(blocklog.ViewGetRequestReceipt.Message(reqID))
	require.NoError(ch.Env.T, err)
	rec, err := blocklog.ViewGetRequestReceipt.Output.Decode(ret)
	require.NoError(ch.Env.T, err)
	return rec, rec != nil
}

// GetRequestReceiptsForBlock returns all request log records for a particular block
func (ch *Chain) GetRequestReceiptsForBlock(blockIndex ...uint32) []*blocklog.RequestReceipt {
	res, err := ch.CallView(blocklog.ViewGetRequestReceiptsForBlock.Message(coreutil.Optional(blockIndex...)))
	if err != nil {
		return nil
	}
	recs, err := blocklog.ViewGetRequestReceiptsForBlock.Output2.Decode(res)
	if err != nil {
		ch.Log().LogWarn(err.Error())
		return nil
	}
	return recs
}

// GetRequestIDsForBlock returns the list of requestIDs settled in a particular block
func (ch *Chain) GetRequestIDsForBlock(blockIndex uint32) []isc.RequestID {
	res, err := ch.CallView(blocklog.ViewGetRequestIDsForBlock.Message(&blockIndex))
	require.NoError(ch.Env.T, err)
	return lo.Must(blocklog.ViewGetRequestIDsForBlock.Output2.Decode(res))
}

// GetRequestReceiptsForBlockRange returns all request log records for range of blocks, inclusively.
// Upper bound is 'latest block' is set to 0
func (ch *Chain) GetRequestReceiptsForBlockRange(fromBlockIndex, toBlockIndex uint32) []*blocklog.RequestReceipt {
	if toBlockIndex == 0 {
		toBlockIndex = ch.GetLatestBlockInfo().BlockIndex()
	}
	if fromBlockIndex > toBlockIndex {
		return nil
	}
	ret := make([]*blocklog.RequestReceipt, 0)
	for i := fromBlockIndex; i <= toBlockIndex; i++ {
		recs := ch.GetRequestReceiptsForBlock(i)
		require.True(ch.Env.T, i == 0 || len(recs) != 0)
		ret = append(ret, recs...)
	}
	return ret
}

func (ch *Chain) GetRequestReceiptsForBlockRangeAsStrings(fromBlockIndex, toBlockIndex uint32) []string {
	recs := ch.GetRequestReceiptsForBlockRange(fromBlockIndex, toBlockIndex)
	ret := make([]string, len(recs))
	for i := range ret {
		ret[i] = recs[i].String()
	}
	return ret
}

func (ch *Chain) GetControlAddresses() *isc.ControlAddresses {
	outs, err := ch.LatestChainOutputs(chaintypes.ConfirmedState)
	if err != nil {
		return nil
	}
	anchorOutput := outs.AnchorOutput
	controlAddr := &isc.ControlAddresses{
		StateAddress:     anchorOutput.StateController(),
		GoverningAddress: anchorOutput.GovernorAddress(),
		SinceBlockIndex:  anchorOutput.StateIndex,
	}
	return controlAddr
}

// AddAllowedStateController adds the address to the allowed state controlled address list
func (ch *Chain) AddAllowedStateController(addr iotago.Address, keyPair *cryptolib.KeyPair) error {
	req := NewCallParams(governance.FuncAddAllowedStateControllerAddress.Message(addr)).
		WithMaxAffordableGasBudget()
	_, err := ch.PostRequestSync(req, keyPair)
	return err
}

// AddAllowedStateController adds the address to the allowed state controlled address list
func (ch *Chain) RemoveAllowedStateController(addr iotago.Address, keyPair *cryptolib.KeyPair) error {
	req := NewCallParams(governance.FuncRemoveAllowedStateControllerAddress.Message(addr)).
		WithMaxAffordableGasBudget()
	_, err := ch.PostRequestSync(req, keyPair)
	return err
}

// AddAllowedStateController adds the address to the allowed state controlled address list
func (ch *Chain) GetAllowedStateControllerAddresses() []iotago.Address {
	res, err := ch.CallView(governance.ViewGetAllowedStateControllerAddresses.Message())
	require.NoError(ch.Env.T, err)
	return lo.Must(governance.ViewGetAllowedStateControllerAddresses.Output.Decode(res))
}

// RotateStateController rotates the chain to the new controller address.
// We assume self-governed chain here.
// Mostly use for the testing of committee rotation logic, otherwise not much needed for smart contract testing
func (ch *Chain) RotateStateController(newStateAddr iotago.Address, newStateKeyPair, ownerKeyPair *cryptolib.KeyPair) error {
	req := NewCallParams(governance.FuncRotateStateController.Message(newStateAddr)).
		WithMaxAffordableGasBudget()
	result := ch.postRequestSyncTxSpecial(req, ownerKeyPair)
	if result.Receipt.Error == nil {
		ch.StateControllerAddress = newStateAddr
		ch.StateControllerKeyPair = newStateKeyPair
	}
	return ch.ResolveVMError(result.Receipt.Error).AsGoError()
}

func (ch *Chain) postRequestSyncTxSpecial(req *CallParams, keyPair *cryptolib.KeyPair) *vm.RequestResult {
	tx, _, err := ch.RequestFromParamsToLedger(req, keyPair)
	require.NoError(ch.Env.T, err)
	reqs, err := ch.Env.RequestsForChain(tx.Transaction, ch.ChainID)
	require.NoError(ch.Env.T, err)
	results := ch.RunRequestsSync(reqs, "postSpecial")
	return results[0]
}

type L1L2AddressAssets struct {
	Address  iotago.Address
	AssetsL1 *isc.Assets
	AssetsL2 *isc.Assets
}

func (a *L1L2AddressAssets) String() string {
	return fmt.Sprintf("Address: %s\nL1 ftokens:\n  %s\nL2 ftokens:\n  %s", a.Address, a.AssetsL1, a.AssetsL2)
}

func (ch *Chain) L1L2Funds(addr iotago.Address) *L1L2AddressAssets {
	return &L1L2AddressAssets{
		Address:  addr,
		AssetsL1: ch.Env.L1Assets(addr),
		AssetsL2: ch.L2Assets(isc.NewAgentID(addr)),
	}
}

func (ch *Chain) GetL2FundsFromFaucet(agentID isc.AgentID, baseTokens ...iotago.BaseToken) {
	// deterministically find a L1 address that has 0 balance on L2
	walletKey, walletAddr := func() (*cryptolib.KeyPair, iotago.Address) {
		masterSeed := []byte("GetL2FundsFromFaucet")
		i := uint32(0)
		for {
			ss := cryptolib.SubSeed(masterSeed, i, testutil.L1API.ProtocolParameters().Bech32HRP())
			key, addr := ch.Env.NewKeyPair(&ss)
			_, err := ch.Env.GetFundsFromFaucet(addr)
			require.NoError(ch.Env.T, err)
			if ch.L2BaseTokens(isc.NewAgentID(addr)) == 0 {
				return key, addr
			}
			i++
		}
	}()

	var amount iotago.BaseToken
	if len(baseTokens) > 0 {
		amount = baseTokens[0]
	} else {
		amount = ch.Env.L1BaseTokens(walletAddr)/2 - TransferAllowanceToGasBudgetBaseTokens
	}
	err := ch.TransferAllowanceTo(
		isc.NewAssetsBaseTokens(amount),
		agentID,
		walletKey,
	)
	require.NoError(ch.Env.T, err)
}

// AttachToRequestProcessed implements chain.Chain
func (*Chain) AttachToRequestProcessed(func(isc.RequestID)) context.CancelFunc {
	panic("unimplemented")
}

// ResolveError implements chain.Chain
func (ch *Chain) ResolveError(e *isc.UnresolvedVMError) (*isc.VMError, error) {
	return ch.ResolveVMError(e), nil
}

// ConfigUpdated implements chain.Chain
func (*Chain) ConfigUpdated(accessNodes []*cryptolib.PublicKey) {
	panic("unimplemented")
}

// ServersUpdated implements chain.Chain
func (*Chain) ServersUpdated(serverNodes []*cryptolib.PublicKey) {
	panic("unimplemented")
}

// GetChainMetrics implements chain.Chain
func (ch *Chain) GetChainMetrics() *metrics.ChainMetrics {
	return ch.metrics
}

// GetConsensusPipeMetrics implements chain.Chain
func (*Chain) GetConsensusPipeMetrics() chaintypes.ConsensusPipeMetrics {
	panic("unimplemented")
}

// GetConsensusWorkflowStatus implements chain.Chain
func (*Chain) GetConsensusWorkflowStatus() chaintypes.ConsensusWorkflowStatus {
	panic("unimplemented")
}

// Store implements chain.Chain
func (ch *Chain) Store() indexedstore.IndexedStore {
	return ch.store
}

// GetTimeData implements chain.Chain
func (*Chain) GetTimeData() time.Time {
	panic("unimplemented")
}

// LatestChainOutputs implements chain.Chain
func (ch *Chain) LatestChainOutputs(freshness chaintypes.StateFreshness) (*isc.ChainOutputs, error) {
	return ch.GetChainOutputsFromL1(), nil
}

// LatestState implements chain.Chain
func (ch *Chain) LatestState(freshness chaintypes.StateFreshness) (state.State, error) {
	if freshness == chaintypes.ActiveOrCommittedState || freshness == chaintypes.ActiveState {
		return ch.store.LatestState()
	}
	outs := ch.GetChainOutputsFromL1()
	l1c, err := transaction.L1CommitmentFromAnchorOutput(outs.AnchorOutput)
	if err != nil {
		panic(err)
	}
	st, err := ch.store.StateByTrieRoot(l1c.TrieRoot())
	if err != nil {
		panic(err)
	}
	return st, nil
}

func (ch *Chain) LatestBlock() state.Block {
	b, err := ch.store.LatestBlock()
	require.NoError(ch.Env.T, err)
	return b
}

func (ch *Chain) Nonce(agentID isc.AgentID) uint64 {
	if evmAgentID, ok := agentID.(*isc.EthereumAddressAgentID); ok {
		nonce, err := ch.EVM().TransactionCount(evmAgentID.EthAddress(), nil)
		require.NoError(ch.Env.T, err)
		return nonce
	}
	res, err := ch.CallView(accounts.ViewGetAccountNonce.Message(&agentID))
	require.NoError(ch.Env.T, err)
	return lo.Must(accounts.ViewGetAccountNonce.Output.Decode(res))
}

// ReceiveOffLedgerRequest implements chain.Chain
func (ch *Chain) ReceiveOffLedgerRequest(request isc.OffLedgerRequest, sender *cryptolib.PublicKey) error {
	_, err := ch.RunOffLedgerRequest(request)
	return err
}

// AwaitRequestProcessed implements chain.Chain
func (ch *Chain) AwaitRequestProcessed(ctx context.Context, requestID isc.RequestID, confirmed bool) <-chan *blocklog.RequestReceipt {
	receiptCh := make(chan *blocklog.RequestReceipt, 1)
	ch.WaitUntil(func() bool {
		return ch.IsRequestProcessed(requestID)
	})
	rec, ok := ch.GetRequestReceipt(requestID)
	require.True(ch.Env.T, ok)
	receiptCh <- rec
	return receiptCh
}

func (ch *Chain) LatestBlockIndex() uint32 {
	return ch.GetLatestBlockInfo().BlockIndex()
}

func (ch *Chain) GetMempoolContents() io.Reader {
	panic("unimplemented")
}

func (ch *Chain) L1APIProvider() iotago.APIProvider {
	return ch.Env.L1APIProvider()
}

func (ch *Chain) TokenInfo() *api.InfoResBaseToken {
	return ch.Env.TokenInfo()
}
