// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

// in the blocklog core contract the VM keeps indices of blocks and requests in an optimized way
// for fast checking and timestamp access.
package governance

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

var Contract = coreutil.NewContract(coreutil.CoreContractGovernance)

var (
	// state controller (entity that owns the state output via AccountAddress)
	FuncRotateStateController = coreutil.NewEP1(Contract, coreutil.CoreEPRotateStateController,
		ParamStateControllerAddress, codec.Address,
	)
	FuncAddAllowedStateControllerAddress = coreutil.NewEP1(Contract, "addAllowedStateControllerAddress",
		ParamStateControllerAddress, codec.Address,
	)
	FuncRemoveAllowedStateControllerAddress = coreutil.NewEP1(Contract, "removeAllowedStateControllerAddress",
		ParamStateControllerAddress, codec.Address,
	)
	ViewGetAllowedStateControllerAddresses = EPViewGetAllowedStateControllerAddresses{
		EP0: coreutil.NewViewEP0(Contract, "getAllowedStateControllerAddresses"),
	}

	// chain owner (L1 entity that is the "owner of the chain")
	FuncClaimChainOwnership    = coreutil.NewEP0(Contract, "claimChainOwnership")
	FuncDelegateChainOwnership = coreutil.NewEP1(Contract, "delegateChainOwnership",
		ParamChainOwner, codec.AgentID,
	)
	FuncSetPayoutAgentID = coreutil.NewEP1(Contract, "setPayoutAgentID",
		ParamSetPayoutAgentID, codec.AgentID,
	)
	FuncSetMinCommonAccountBalance = coreutil.NewEP1(Contract, "setMinCommonAccountBalance",
		ParamSetMinCommonAccountBalance, codec.Uint64,
	)
	ViewGetPayoutAgentID = coreutil.NewViewEP01(Contract, "getPayoutAgentID",
		ParamSetPayoutAgentID, codec.AgentID,
	)
	ViewGetMinCommonAccountBalance = coreutil.NewViewEP01(Contract, "getMinCommonAccountBalance",
		ParamSetMinCommonAccountBalance, codec.Uint64,
	)
	ViewGetChainOwner = coreutil.NewViewEP01(Contract, "getChainOwner",
		ParamChainOwner, codec.AgentID,
	)

	// gas
	FuncSetFeePolicy = coreutil.NewEP1(Contract, "setFeePolicy",
		ParamFeePolicyBytes, codec.NewCodecEx(gas.FeePolicyFromBytes),
	)
	FuncSetGasLimits = coreutil.NewEP1(Contract, "setGasLimits",
		ParamGasLimitsBytes, codec.NewCodecEx(gas.LimitsFromBytes),
	)
	ViewGetFeePolicy = coreutil.NewViewEP01(Contract, "getFeePolicy",
		ParamFeePolicyBytes, codec.NewCodecEx(gas.FeePolicyFromBytes),
	)
	ViewGetGasLimits = coreutil.NewViewEP01(Contract, "getGasLimits",
		ParamGasLimitsBytes, codec.NewCodecEx(gas.LimitsFromBytes),
	)

	// evm fees
	FuncSetEVMGasRatio = coreutil.NewEP1(Contract, "setEVMGasRatio",
		ParamEVMGasRatio, codec.NewCodecEx(util.Ratio32FromBytes),
	)
	ViewGetEVMGasRatio = coreutil.NewViewEP01(Contract, "getEVMGasRatio",
		ParamEVMGasRatio, codec.NewCodecEx(util.Ratio32FromBytes),
	)

	// chain info
	ViewGetChainInfo = EPViewGetChainInfo{EP0: coreutil.NewViewEP0(Contract, "getChainInfo")}

	// access nodes
	FuncAddCandidateNode  = EPAddCandidateNode{EntryPointInfo: Contract.Func("addCandidateNode")}
	FuncRevokeAccessNode  = EPRevokeAccessNode{EntryPointInfo: Contract.Func("revokeAccessNode")}
	FuncChangeAccessNodes = EPChangeAccessNodes{EntryPointInfo: Contract.Func("changeAccessNodes")}
	ViewGetChainNodes     = EPGetChainNodes{EP0: coreutil.NewViewEP0(Contract, "getChainNodes")}

	// maintenance
	FuncStartMaintenance     = coreutil.NewEP0(Contract, "startMaintenance")
	FuncStopMaintenance      = coreutil.NewEP0(Contract, "stopMaintenance")
	ViewGetMaintenanceStatus = coreutil.NewViewEP01(Contract, "getMaintenanceStatus",
		VarMaintenanceStatus, codec.Bool,
	)

	// public chain metadata
	FuncSetMetadata = coreutil.NewEP2(Contract, "setMetadata",
		ParamPublicURL, codec.String,
		ParamMetadata, codec.Bytes,
	)
	ViewGetMetadata = coreutil.NewViewEP02(Contract, "getMetadata",
		ParamPublicURL, codec.String,
		ParamMetadata, codec.Bytes,
	)
)

// state variables
const (
	// state controller
	VarAllowedStateControllerAddresses = "a"
	VarRotateToAddress                 = "r"

	VarPayoutAgentID                = "pa"
	VarMinBaseTokensOnCommonAccount = "vs"

	// chain owner
	VarChainOwnerID          = "o"
	VarChainOwnerIDDelegated = "n"

	// gas
	VarGasFeePolicyBytes = "g"
	VarGasLimitsBytes    = "l"

	// access nodes
	VarAccessNodes          = "an"
	VarAccessNodeCandidates = "ac"

	// maintenance
	VarMaintenanceStatus = "m"

	// L2 metadata (provided by the webapi, located by the public url)
	VarMetadata = "md"

	// L1 metadata (stored and provided in the tangle)
	VarPublicURL = "x"

	// state pruning
	VarBlockKeepAmount = "b"

	// AccountID of the chain's AccountOutput
	VarChainAccountID = "A"
)

// request parameters
const (
	// state controller
	ParamStateControllerAddress          = coreutil.ParamStateControllerAddress
	ParamAllowedStateControllerAddresses = "a"

	// chain owner
	ParamChainOwner = "o"

	// gas
	ParamFeePolicyBytes = "g"
	ParamEVMGasRatio    = "e"
	ParamGasLimitsBytes = "l"

	// chain info
	ParamChainID = "c"

	ParamGetChainNodesAccessNodeCandidates = "an"
	ParamGetChainNodesAccessNodes          = "ac"

	// access nodes: addCandidateNode
	ParamAccessNodeInfoForCommittee = "i"
	ParamAccessNodeInfoPubKey       = "ip"
	ParamAccessNodeInfoCertificate  = "ic"
	ParamAccessNodeInfoAccessAPI    = "ia"

	// access nodes: changeAccessNodes
	ParamChangeAccessNodesActions = "n"

	// public chain metadata (provided by the webapi, located by the public url)
	ParamMetadata = "md"

	// L1 metadata (stored and provided in the tangle)
	ParamPublicURL = "x"

	// state pruning
	ParamBlockKeepAmount = "b"

	// set payout AgentID
	ParamSetPayoutAgentID = "s"

	// set min SD
	ParamSetMinCommonAccountBalance = "ms"
)

// contract constants
const (
	// DefaultMinBaseTokensOnCommonAccount can't harvest the minimum
	DefaultMinBaseTokensOnCommonAccount = iotago.BaseToken(3000)

	BlockKeepAll           = -1
	DefaultBlockKeepAmount = 10_000
)

type EPViewGetAllowedStateControllerAddresses struct {
	coreutil.EP0[isc.SandboxView]
	Output FieldAddressList
}

type FieldAddressList struct{}

func (e FieldAddressList) Decode(r dict.Dict) ([]iotago.Address, error) {
	if len(r) == 0 {
		return nil, nil
	}
	addresses := collections.NewArrayReadOnly(r, ParamAllowedStateControllerAddresses)
	ret := make([]iotago.Address, addresses.Len())
	for i := range ret {
		var err error
		ret[i], err = codec.Address.Decode(addresses.GetAt(uint32(i)))
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

type EPViewGetChainInfo struct {
	coreutil.EP0[isc.SandboxView]
	Output GetChainInfoOutput
}

type GetChainInfoOutput struct{}

func (o GetChainInfoOutput) Decode(r dict.Dict, chainID isc.ChainID) (*isc.ChainInfo, error) {
	return GetChainInfo(r, chainID)
}

type EPAddCandidateNode struct {
	coreutil.EntryPointInfo[isc.Sandbox]
}

func (e EPAddCandidateNode) Message(a *AccessNodeInfo) isc.Message {
	return e.EntryPointInfo.Message(a.ToAddCandidateNodeParams())
}

type EPRevokeAccessNode struct {
	coreutil.EntryPointInfo[isc.Sandbox]
}

func (e EPRevokeAccessNode) Message(a *AccessNodeInfo) isc.Message {
	return e.EntryPointInfo.Message(a.ToRevokeAccessNodeParams())
}

type EPChangeAccessNodes struct {
	coreutil.EntryPointInfo[isc.Sandbox]
}

func (e EPChangeAccessNodes) Message(r *ChangeAccessNodesRequest) isc.Message {
	return e.EntryPointInfo.Message(r.AsDict())
}

type EPGetChainNodes struct {
	coreutil.EP0[isc.SandboxView]
	Output GetChainNodesOutput
}

type GetChainNodesOutput struct{}

func (o GetChainNodesOutput) Decode(r dict.Dict) *GetChainNodesResponse {
	return getChainNodesResponseFromDict(r)
}
