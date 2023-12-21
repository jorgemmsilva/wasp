// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/kv/kvdecoder"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

// GetRotationAddress tries to read the state of 'governance' and extract rotation address
// If succeeds, it means this block is fake.
// If fails, return nil
func GetRotationAddress(state kv.KVStoreReader) iotago.Address {
	ret, err := codec.Address.Decode(state.Get(VarRotateToAddress), nil)
	if err != nil {
		return nil
	}
	return ret
}

// GetChainInfo returns global variables of the chain
func GetChainInfo(state kv.KVStoreReader, chainID isc.ChainID) (*isc.ChainInfo, error) {
	d := kvdecoder.New(state)
	ret := &isc.ChainInfo{
		ChainID:  chainID,
		Metadata: &isc.PublicChainMetadata{},
	}
	var err error
	if ret.ChainOwnerID, err = d.GetAgentID(VarChainOwnerID); err != nil {
		return nil, err
	}

	if ret.GasFeePolicy, err = GetGasFeePolicy(state); err != nil {
		return nil, err
	}

	if ret.GasLimits, err = GetGasLimits(state); err != nil {
		return nil, err
	}

	ret.BlockKeepAmount = GetBlockKeepAmount(state)
	if ret.PublicURL, err = GetPublicURL(state); err != nil {
		return nil, err
	}

	if ret.Metadata, err = GetMetadata(state); err != nil {
		return nil, err
	}

	ret.ChainAccountID, _ = GetChainAccountID(state)

	return ret, nil
}

func GetMinCommonAccountBalance(state kv.KVStoreReader) iotago.BaseToken {
	return iotago.BaseToken(kvdecoder.New(state).MustGetUint64(VarMinBaseTokensOnCommonAccount))
}

func GetPayoutAgentID(state kv.KVStoreReader) isc.AgentID {
	return kvdecoder.New(state).MustGetAgentID(VarPayoutAgentID)
}

func mustGetChainOwnerID(state kv.KVStoreReader) isc.AgentID {
	d := kvdecoder.New(state)
	return d.MustGetAgentID(VarChainOwnerID)
}

// GetGasFeePolicy returns gas policy from the state
func GetGasFeePolicy(state kv.KVStoreReader) (*gas.FeePolicy, error) {
	return gas.FeePolicyFromBytes(state.Get(VarGasFeePolicyBytes))
}

func GetGasLimits(state kv.KVStoreReader) (*gas.Limits, error) {
	data := state.Get(VarGasLimitsBytes)
	if data == nil {
		return gas.LimitsDefault, nil
	}
	return gas.LimitsFromBytes(data)
}

func GetBlockKeepAmount(state kv.KVStoreReader) int32 {
	return lo.Must(codec.Int32.Decode(state.Get(VarBlockKeepAmount), DefaultBlockKeepAmount))
}

func SetPublicURL(state kv.KVStore, url string) {
	state.Set(VarPublicURL, codec.String.Encode(url))
}

func GetPublicURL(state kv.KVStoreReader) (string, error) {
	return codec.String.Decode(state.Get(VarPublicURL), "")
}

func SetMetadata(state kv.KVStore, metadata *isc.PublicChainMetadata) {
	state.Set(VarMetadata, metadata.Bytes())
}

func GetMetadata(state kv.KVStoreReader) (*isc.PublicChainMetadata, error) {
	metadataBytes := state.Get(VarMetadata)
	if metadataBytes == nil {
		return &isc.PublicChainMetadata{}, nil
	}
	return isc.PublicChainMetadataFromBytes(metadataBytes)
}

func SetChainAccountID(state kv.KVStore, accountID iotago.AccountID) {
	state.Set(VarChainAccountID, accountID[:])
}

func GetChainAccountID(state kv.KVStoreReader) (iotago.AccountID, bool) {
	b := state.Get(VarChainAccountID)
	if b == nil {
		return iotago.AccountID{}, false
	}
	ret, _, err := iotago.AccountIDFromBytes(b)
	if err != nil {
		panic(err)
	}
	return ret, true
}

func AccessNodesMap(state kv.KVStore) *collections.Map {
	return collections.NewMap(state, VarAccessNodes)
}

func AccessNodesMapR(state kv.KVStoreReader) *collections.ImmutableMap {
	return collections.NewMapReadOnly(state, VarAccessNodes)
}

func AccessNodeCandidatesMap(state kv.KVStore) *collections.Map {
	return collections.NewMap(state, VarAccessNodeCandidates)
}

func AccessNodeCandidatesMapR(state kv.KVStoreReader) *collections.ImmutableMap {
	return collections.NewMapReadOnly(state, VarAccessNodeCandidates)
}
