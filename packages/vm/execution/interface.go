package execution

import (
	"math/big"
	"time"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/packages/vm/gas"
	"github.com/iotaledger/wasp/packages/vm/processors"
)

// WaspContext defines the common functionality for vm context - both used in internal/external calls (SC execution/external view calls)
type WaspContext interface {
	LocateProgram(programHash hashing.HashValue) (vmtype string, binary []byte, err error)
	GetContractRecord(contractHname isc.Hname) (ret *root.ContractRecord)
	GasBurnEnable(enable bool)
	GasBurn(burnCode gas.BurnCode, par ...uint64)
	Processors() *processors.Cache

	// needed for sandbox
	isc.LogInterface
	Timestamp() time.Time
	AccountID() isc.AgentID
	Caller() isc.AgentID
	GetNativeTokens(agentID isc.AgentID) iotago.NativeTokens
	GetBaseTokensBalance(agentID isc.AgentID) uint64
	GetNativeTokenBalance(agentID isc.AgentID, nativeTokenID iotago.NativeTokenID) *big.Int
	Call(contractHname, entryPoint isc.Hname, params dict.Dict, allowance *isc.Assets) []byte
	ChainID() isc.ChainID
	ChainOwnerID() isc.AgentID
	CurrentContractHname() isc.Hname
	Params() *isc.Params
	StateReader() kv.KVStoreReader
	GasBurned() uint64
	GasBudgetLeft() uint64
	GetAccountNFTs(agentID isc.AgentID) []iotago.NFTID
	GetNFTData(nftID iotago.NFTID) *isc.NFT
}
