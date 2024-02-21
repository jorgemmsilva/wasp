package vmimpl

import (
	"time"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/corecontracts"
	"github.com/iotaledger/wasp/packages/vm/core/errors"
	"github.com/iotaledger/wasp/packages/vm/core/root"
)

func (reqctx *requestContext) ChainID() isc.ChainID {
	return reqctx.vm.ChainID()
}

func (vmctx *vmContext) ChainID() isc.ChainID {
	if vmctx.task.Inputs.AnchorOutput.StateIndex == 0 {
		// origin
		return isc.ChainIDFromAnchorID(iotago.AnchorIDFromOutputID(vmctx.task.Inputs.AnchorOutputID))
	}
	return isc.ChainIDFromAnchorID(vmctx.task.Inputs.AnchorOutput.AnchorID)
}

func (reqctx *requestContext) ChainAccountID() (iotago.AccountID, bool) {
	return reqctx.vm.ChainAccountID()
}

func (vmctx *vmContext) MustChainAccountID() iotago.AccountID {
	accountiD, ok := vmctx.ChainAccountID()
	if !ok {
		panic("chain AccountID unknown")
	}
	return accountiD
}

func (vmctx *vmContext) ChainAccountID() (iotago.AccountID, bool) {
	id, out, ok := vmctx.task.Inputs.AccountOutput()
	if !ok {
		return iotago.AccountID{}, false
	}
	return util.AccountIDFromAccountOutput(out, id), true
}

func (reqctx *requestContext) ChainInfo() *isc.ChainInfo {
	return reqctx.vm.ChainInfo()
}

func (vmctx *vmContext) ChainInfo() *isc.ChainInfo {
	return vmctx.chainInfo
}

func (reqctx *requestContext) ChainOwnerID() isc.AgentID {
	return reqctx.vm.ChainOwnerID()
}

func (vmctx *vmContext) ChainOwnerID() isc.AgentID {
	return vmctx.chainInfo.ChainOwnerID
}

func (reqctx *requestContext) CurrentContractAgentID() isc.AgentID {
	return isc.NewContractAgentID(reqctx.vm.ChainID(), reqctx.CurrentContractHname())
}

func (reqctx *requestContext) CurrentContractHname() isc.Hname {
	return reqctx.getCallContext().contract
}

func (reqctx *requestContext) Params() *isc.Params {
	return &reqctx.getCallContext().params
}

func (reqctx *requestContext) Caller() isc.AgentID {
	return reqctx.getCallContext().caller
}

func (reqctx *requestContext) Timestamp() time.Time {
	return reqctx.vm.task.Timestamp
}

func (reqctx *requestContext) CurrentContractAccountID() isc.AgentID {
	hname := reqctx.CurrentContractHname()
	if corecontracts.IsCoreHname(hname) {
		return accounts.CommonAccount()
	}
	return isc.NewContractAgentID(reqctx.vm.ChainID(), hname)
}

func (reqctx *requestContext) allowanceAvailable() *isc.Assets {
	return reqctx.getCallContext().allowanceAvailable.Clone()
}

func (vmctx *vmContext) isCoreAccount(agentID isc.AgentID) bool {
	contract, ok := agentID.(*isc.ContractAgentID)
	if !ok {
		return false
	}
	return contract.ChainID().Equals(vmctx.ChainID()) && corecontracts.IsCoreHname(contract.Hname())
}

func (reqctx *requestContext) spendAllowedBudget(toSpend *isc.Assets) {
	if !reqctx.getCallContext().allowanceAvailable.Spend(toSpend) {
		panic(accounts.ErrNotEnoughAllowance)
	}
}

// TransferAllowedFunds transfers funds within the budget set by the Allowance() to the existing target account on chain
func (reqctx *requestContext) transferAllowedFunds(target isc.AgentID, transfer ...*isc.Assets) *isc.Assets {
	if reqctx.vm.isCoreAccount(target) {
		// if the target is one of core contracts, assume target is the common account
		target = accounts.CommonAccount()
	}

	var toMove *isc.Assets
	if len(transfer) == 0 {
		toMove = reqctx.allowanceAvailable()
	} else {
		toMove = transfer[0]
	}

	reqctx.spendAllowedBudget(toMove) // panics if not enough

	caller := reqctx.Caller() // have to take it here because callCore changes that

	// if the caller is a core contract, funds should be taken from the common account
	if reqctx.vm.isCoreAccount(caller) {
		caller = accounts.CommonAccount()
	}
	reqctx.callAccounts(func(s *accounts.StateWriter) {
		if err := s.MoveBetweenAccounts(caller, target, toMove); err != nil {
			panic(vm.ErrNotEnoughFundsForAllowance)
		}
	})
	return reqctx.allowanceAvailable()
}

func (vmctx *vmContext) stateAnchor() *isc.StateAnchor {
	blockset := vmctx.task.Inputs.AnchorOutput.FeatureSet()
	senderBlock := blockset.SenderFeature()
	var sender iotago.Address
	if senderBlock != nil {
		sender = senderBlock.Address
	}
	stateData, err := transaction.StateMetadataBytesFromAnchorOutput(vmctx.task.Inputs.AnchorOutput)
	if err != nil {
		panic(err)
	}
	return &isc.StateAnchor{
		ChainID:              vmctx.ChainID(),
		Sender:               sender,
		IsOrigin:             vmctx.task.Inputs.AnchorOutput.AnchorID == iotago.EmptyAnchorID,
		StateController:      vmctx.task.Inputs.AnchorOutput.StateController(),
		GovernanceController: vmctx.task.Inputs.AnchorOutput.GovernorAddress(),
		StateIndex:           vmctx.task.Inputs.AnchorOutput.StateIndex,
		OutputID:             vmctx.task.Inputs.AnchorOutputID,
		StateData:            stateData,
		Deposit:              vmctx.task.Inputs.AnchorOutput.Amount,
	}
}

// DeployContract deploys contract by its program hash with the name specific to the instance
func (reqctx *requestContext) deployContract(programHash hashing.HashValue, name string, initParams dict.Dict) {
	reqctx.LogDebugf("vmcontext.DeployContract: %s, name: %s", programHash.String(), name)
	reqctx.Call(root.FuncDeployContract.Message(name, programHash, initParams), nil)
}

func (reqctx *requestContext) registerError(messageFormat string) *isc.VMErrorTemplate {
	reqctx.LogDebugf("vmcontext.RegisterError: messageFormat: '%s'", messageFormat)
	result := reqctx.Call(errors.FuncRegisterError.Message(messageFormat), nil)
	errorCode := lo.Must(codec.VMErrorCode.Decode(result.Get(errors.ParamErrorCode)))

	reqctx.LogDebugf("vmcontext.RegisterError: errorCode: '%s'", errorCode)

	return isc.NewVMErrorTemplate(errorCode, messageFormat)
}

func (reqctx *requestContext) L1API() iotago.API {
	return reqctx.vm.task.L1API()
}

func (reqctx *requestContext) L1APIProvider() iotago.APIProvider {
	return reqctx.vm.task.L1APIProvider
}

func (reqctx *requestContext) TokenInfo() *api.InfoResBaseToken {
	return reqctx.vm.task.TokenInfo
}
