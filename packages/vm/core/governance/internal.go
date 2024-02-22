// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

func (s *StateWriter) SetInitialState(chainOwner isc.AgentID, blockKeepAmount int32) {
	s.SetChainOwnerID(chainOwner)
	s.SetGasFeePolicy(gas.DefaultFeePolicy())
	s.SetGasLimits(gas.LimitsDefault)
	s.SetMaintenanceStatus(false)
	s.SetBlockKeepAmount(blockKeepAmount)
	s.SetMinCommonAccountBalance(DefaultMinBaseTokensOnCommonAccount)
	s.SetPayoutAgentID(chainOwner)
}

// GetRotationAddress tries to read the state of 'governance' and extract rotation address
// If succeeds, it means this block is fake.
// If fails, return nil
func (s *StateReader) GetRotationAddress() iotago.Address {
	ret, err := codec.Address.Decode(s.state.Get(VarRotateToAddress), nil)
	if err != nil {
		return nil
	}
	return ret
}

// GetChainInfo returns global variables of the chain
func (s *StateReader) GetChainInfo(chainID isc.ChainID) (*isc.ChainInfo, error) {
	ret := &isc.ChainInfo{
		ChainID:  chainID,
		Metadata: &isc.PublicChainMetadata{},
	}
	var err error
	ret.ChainOwnerID = s.GetChainOwnerID()

	if ret.GasFeePolicy, err = s.GetGasFeePolicy(); err != nil {
		return nil, err
	}

	if ret.GasLimits, err = s.GetGasLimits(); err != nil {
		return nil, err
	}

	ret.BlockKeepAmount = s.GetBlockKeepAmount()
	if ret.PublicURL, err = s.GetPublicURL(); err != nil {
		return nil, err
	}

	if ret.Metadata, err = s.GetMetadata(); err != nil {
		return nil, err
	}

	ret.ChainAccountID, _ = s.GetChainAccountID()

	return ret, nil
}

func (s *StateReader) GetMinCommonAccountBalance() iotago.BaseToken {
	return lo.Must(codec.BaseToken.Decode(s.state.Get(VarMinBaseTokensOnCommonAccount)))
}

func (s *StateWriter) SetMinCommonAccountBalance(m iotago.BaseToken) {
	s.state.Set(VarMinBaseTokensOnCommonAccount, codec.BaseToken.Encode(m))
}

func (s *StateReader) GetChainOwnerID() isc.AgentID {
	return lo.Must(codec.AgentID.Decode(s.state.Get(VarChainOwnerID)))
}

func (s *StateWriter) SetChainOwnerID(a isc.AgentID) {
	s.state.Set(VarChainOwnerID, codec.AgentID.Encode(a))
	if s.GetChainOwnerIDDelegated() != nil {
		s.state.Del(VarChainOwnerIDDelegated)
	}
}

func (s *StateReader) GetChainOwnerIDDelegated() isc.AgentID {
	return lo.Must(codec.AgentID.Decode(s.state.Get(VarChainOwnerIDDelegated), nil))
}

func (s *StateWriter) SetChainOwnerIDDelegated(a isc.AgentID) {
	s.state.Set(VarChainOwnerIDDelegated, codec.AgentID.Encode(a))
}

func (s *StateReader) GetPayoutAgentID() isc.AgentID {
	return lo.Must(codec.AgentID.Decode(s.state.Get(VarPayoutAgentID)))
}

func (s *StateWriter) SetPayoutAgentID(a isc.AgentID) {
	s.state.Set(VarPayoutAgentID, codec.AgentID.Encode(a))
}

// GetGasFeePolicy returns gas policy from the state
func (s *StateReader) GetGasFeePolicy() (*gas.FeePolicy, error) {
	return gas.FeePolicyFromBytes(s.state.Get(VarGasFeePolicyBytes))
}

func (s *StateWriter) SetGasFeePolicy(fp *gas.FeePolicy) {
	s.state.Set(VarGasFeePolicyBytes, fp.Bytes())
}

func (s *StateReader) GetGasLimits() (*gas.Limits, error) {
	data := s.state.Get(VarGasLimitsBytes)
	if data == nil {
		return gas.LimitsDefault, nil
	}
	return gas.LimitsFromBytes(data)
}

func (s *StateWriter) SetGasLimits(gl *gas.Limits) {
	s.state.Set(VarGasLimitsBytes, gl.Bytes())
}

func (s *StateReader) GetBlockKeepAmount() int32 {
	return lo.Must(codec.Int32.Decode(s.state.Get(VarBlockKeepAmount), DefaultBlockKeepAmount))
}

func (s *StateWriter) SetBlockKeepAmount(n int32) {
	s.state.Set(VarBlockKeepAmount, codec.Int32.Encode(n))
}

func (s *StateWriter) SetPublicURL(url string) {
	s.state.Set(VarPublicURL, codec.String.Encode(url))
}

func (s *StateReader) GetPublicURL() (string, error) {
	return codec.String.Decode(s.state.Get(VarPublicURL), "")
}

func (s *StateWriter) SetMetadata(metadata *isc.PublicChainMetadata) {
	s.state.Set(VarMetadata, metadata.Bytes())
}

func (s *StateReader) GetMetadata() (*isc.PublicChainMetadata, error) {
	metadataBytes := s.state.Get(VarMetadata)
	if metadataBytes == nil {
		return &isc.PublicChainMetadata{}, nil
	}
	return isc.PublicChainMetadataFromBytes(metadataBytes)
}

func (s *StateWriter) SetChainAccountID(accountID iotago.AccountID) {
	s.state.Set(VarChainAccountID, accountID[:])
}

func (s *StateReader) GetChainAccountID() (iotago.AccountID, bool) {
	b := s.state.Get(VarChainAccountID)
	if b == nil {
		return iotago.AccountID{}, false
	}
	ret, _, err := iotago.AccountIDFromBytes(b)
	if err != nil {
		panic(err)
	}
	return ret, true
}

func (s *StateWriter) AccessNodesMap() *collections.Map {
	return collections.NewMap(s.state, VarAccessNodes)
}

func (s *StateReader) AccessNodesMap() *collections.ImmutableMap {
	return collections.NewMapReadOnly(s.state, VarAccessNodes)
}

func (s *StateWriter) AccessNodeCandidatesMap() *collections.Map {
	return collections.NewMap(s.state, VarAccessNodeCandidates)
}

func (s *StateReader) AccessNodeCandidatesMap() *collections.ImmutableMap {
	return collections.NewMapReadOnly(s.state, VarAccessNodeCandidates)
}

func (s *StateReader) MaintenanceStatus() bool {
	r := s.state.Get(VarMaintenanceStatus)
	if r == nil {
		return false // chain is being initialized, governance has not been initialized yet
	}
	return lo.Must(codec.Bool.Decode(r))
}

func (s *StateWriter) SetMaintenanceStatus(status bool) {
	s.state.Set(VarMaintenanceStatus, codec.Bool.Encode(status))
}

func (s *StateReader) AccessNodes() []*cryptolib.PublicKey {
	accessNodes := []*cryptolib.PublicKey{}
	s.AccessNodesMap().IterateKeys(func(pubKeyBytes []byte) bool {
		pubKey, err := cryptolib.PublicKeyFromBytes(pubKeyBytes)
		if err != nil {
			panic(err)
		}
		accessNodes = append(accessNodes, pubKey)
		return true
	})
	return accessNodes
}

func (s *StateReader) CandidateNodes() []*AccessNodeInfo {
	candidateNodes := []*AccessNodeInfo{}
	s.AccessNodeCandidatesMap().Iterate(func(pubKeyBytes, accessNodeInfoBytes []byte) bool {
		ani, err := AccessNodeInfoFromBytes(pubKeyBytes, accessNodeInfoBytes)
		if err != nil {
			panic(err)
		}
		candidateNodes = append(candidateNodes, ani)
		return true
	})
	return candidateNodes
}
