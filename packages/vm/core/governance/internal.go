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

func (s *StateWriter) SetRotationAddress(addr iotago.Address) {
	s.state.Set(varRotateToAddress, isc.AddressToBytes(addr))
}

func (s *StateWriter) SetBlockIssuer(accID iotago.AccountID) {
	s.state.Set(varStateControllerBlockIssuer, lo.Must(accID.Bytes()))
}

// GetRotationAddress tries to read the state of 'governance' and extract rotation address
// If succeeds, it means this block is fake.
// If fails, return nil
func (s *StateReader) GetRotationAddress() iotago.Address {
	ret, err := codec.Address.Decode(s.state.Get(varRotateToAddress), nil)
	if err != nil {
		return nil
	}
	return ret
}

func (s *StateReader) GetBlockIssuer() iotago.AccountID {
	ret, err := codec.AccountID.Decode(s.state.Get(varStateControllerBlockIssuer))
	if err != nil {
		return iotago.EmptyAccountID
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
	return lo.Must(codec.BaseToken.Decode(s.state.Get(varMinBaseTokensOnCommonAccount)))
}

func (s *StateWriter) SetMinCommonAccountBalance(m iotago.BaseToken) {
	s.state.Set(varMinBaseTokensOnCommonAccount, codec.BaseToken.Encode(m))
}

func (s *StateReader) GetChainOwnerID() isc.AgentID {
	return lo.Must(codec.AgentID.Decode(s.state.Get(varChainOwnerID)))
}

func (s *StateWriter) SetChainOwnerID(a isc.AgentID) {
	s.state.Set(varChainOwnerID, codec.AgentID.Encode(a))
	if s.GetChainOwnerIDDelegated() != nil {
		s.state.Del(varChainOwnerIDDelegated)
	}
}

func (s *StateReader) GetChainOwnerIDDelegated() isc.AgentID {
	return lo.Must(codec.AgentID.Decode(s.state.Get(varChainOwnerIDDelegated), nil))
}

func (s *StateWriter) SetChainOwnerIDDelegated(a isc.AgentID) {
	s.state.Set(varChainOwnerIDDelegated, codec.AgentID.Encode(a))
}

func (s *StateReader) GetPayoutAgentID() isc.AgentID {
	return lo.Must(codec.AgentID.Decode(s.state.Get(varPayoutAgentID)))
}

func (s *StateWriter) SetPayoutAgentID(a isc.AgentID) {
	s.state.Set(varPayoutAgentID, codec.AgentID.Encode(a))
}

// GetGasFeePolicy returns gas policy from the state
func (s *StateReader) GetGasFeePolicy() (*gas.FeePolicy, error) {
	return gas.FeePolicyFromBytes(s.state.Get(varGasFeePolicyBytes))
}

func (s *StateWriter) SetGasFeePolicy(fp *gas.FeePolicy) {
	s.state.Set(varGasFeePolicyBytes, fp.Bytes())
}

func (s *StateReader) GetGasLimits() (*gas.Limits, error) {
	data := s.state.Get(varGasLimitsBytes)
	if data == nil {
		return gas.LimitsDefault, nil
	}
	return gas.LimitsFromBytes(data)
}

func (s *StateWriter) SetGasLimits(gl *gas.Limits) {
	s.state.Set(varGasLimitsBytes, gl.Bytes())
}

func (s *StateReader) GetBlockKeepAmount() int32 {
	return lo.Must(codec.Int32.Decode(s.state.Get(varBlockKeepAmount), DefaultBlockKeepAmount))
}

func (s *StateWriter) SetBlockKeepAmount(n int32) {
	s.state.Set(varBlockKeepAmount, codec.Int32.Encode(n))
}

func (s *StateWriter) SetPublicURL(url string) {
	s.state.Set(varPublicURL, codec.String.Encode(url))
}

func (s *StateReader) GetPublicURL() (string, error) {
	return codec.String.Decode(s.state.Get(varPublicURL), "")
}

func (s *StateWriter) SetMetadata(metadata *isc.PublicChainMetadata) {
	s.state.Set(varMetadata, metadata.Bytes())
}

func (s *StateReader) GetMetadata() (*isc.PublicChainMetadata, error) {
	metadataBytes := s.state.Get(varMetadata)
	if metadataBytes == nil {
		return &isc.PublicChainMetadata{}, nil
	}
	return isc.PublicChainMetadataFromBytes(metadataBytes)
}

func (s *StateWriter) SetChainAccountID(accountID iotago.AccountID) {
	s.state.Set(varChainAccountID, accountID[:])
}

func (s *StateReader) GetChainAccountID() (iotago.AccountID, bool) {
	b := s.state.Get(varChainAccountID)
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
	return collections.NewMap(s.state, varAccessNodes)
}

func (s *StateReader) AccessNodesMap() *collections.ImmutableMap {
	return collections.NewMapReadOnly(s.state, varAccessNodes)
}

func (s *StateWriter) AccessNodeCandidatesMap() *collections.Map {
	return collections.NewMap(s.state, varAccessNodeCandidates)
}

func (s *StateReader) AccessNodeCandidatesMap() *collections.ImmutableMap {
	return collections.NewMapReadOnly(s.state, varAccessNodeCandidates)
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

func (s *StateWriter) GetAllowedStateControllerAddresses() *collections.Map {
	return collections.NewMap(s.state, varAllowedStateControllerAddresses)
}

func (s *StateReader) GetAllowedStateControllerAddresses() *collections.ImmutableMap {
	return collections.NewMapReadOnly(s.state, varAllowedStateControllerAddresses)
}
