// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package accounts

import (
	"github.com/ethereum/go-ethereum/common"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/subrealm"
)

func ContractState(chainState kv.KVStore) kv.KVStore {
	return subrealm.New(chainState, kv.Key(Contract.Hname().Bytes()))
}

func ContractStateR(chainState kv.KVStoreReader) kv.KVStoreReader {
	return subrealm.NewReadOnly(chainState, kv.Key(Contract.Hname().Bytes()))
}

// StateContext is a subset of SandboxBase
type StateContext interface {
	SchemaVersion() isc.SchemaVersion
	ChainID() isc.ChainID
	TokenInfo() *api.InfoResBaseToken
	L1API() iotago.API
}

type stateContext struct {
	schemaVersion isc.SchemaVersion
	chainID       isc.ChainID
	tokenInfo     *api.InfoResBaseToken
	l1API         iotago.API
}

func NewStateContext(
	schemaVersion isc.SchemaVersion,
	chainID isc.ChainID,
	tokenInfo *api.InfoResBaseToken,
	l1API iotago.API,
) StateContext {
	return &stateContext{
		schemaVersion: schemaVersion,
		chainID:       chainID,
		tokenInfo:     tokenInfo,
		l1API:         l1API,
	}
}

func (ctx *stateContext) ChainID() isc.ChainID {
	return ctx.chainID
}

func (ctx *stateContext) L1API() iotago.API {
	return ctx.l1API
}

func (ctx *stateContext) SchemaVersion() isc.SchemaVersion {
	return ctx.schemaVersion
}

func (ctx *stateContext) TokenInfo() *api.InfoResBaseToken {
	return ctx.tokenInfo
}

var _ StateContext = &stateContext{}

type StateReader struct {
	ctx   StateContext
	state kv.KVStoreReader
}

func NewStateReader(ctx StateContext, contractState kv.KVStoreReader) *StateReader {
	return &StateReader{
		ctx:   ctx,
		state: contractState,
	}
}

func NewStateReaderFromSandbox(ctx isc.SandboxBase) *StateReader {
	return NewStateReader(ctx, ctx.StateR())
}

type StateWriter struct {
	*StateReader
	state kv.KVStore
}

func NewStateWriter(ctx StateContext, contractState kv.KVStore) *StateWriter {
	return &StateWriter{
		StateReader: NewStateReader(ctx, contractState),
		state:       contractState,
	}
}

func NewStateWriterFromSandbox(ctx isc.Sandbox) *StateWriter {
	return NewStateWriter(ctx, ctx.State())
}

// converts an account key from the accounts contract (shortform without chainID) to an AgentID
func agentIDFromKey(key kv.Key, chainID isc.ChainID) (isc.AgentID, error) {
	if len(key) < isc.ChainIDLength {
		// short form saved (withoutChainID)
		switch len(key) {
		case 4:
			hn, err := isc.HnameFromBytes([]byte(key))
			if err != nil {
				return nil, err
			}
			return isc.NewContractAgentID(chainID, hn), nil
		case common.AddressLength:
			var ethAddr common.Address
			copy(ethAddr[:], []byte(key))
			return isc.NewEthereumAddressAgentID(chainID, ethAddr), nil
		default:
			panic("bad key length")
		}
	}
	return codec.AgentID.Decode([]byte(key))
}
