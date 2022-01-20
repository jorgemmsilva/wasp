// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package sandbox

import (
	"math/big"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/iscp/assert"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/vm/gas"
	"github.com/iotaledger/wasp/packages/vm/vmcontext"
)

type sandbox struct {
	vmctx  *vmcontext.VMContext
	assert *assert.Assert
}

var _ iscp.Sandbox = &sandbox{}

func init() {
	vmcontext.NewSandbox = func(vmctx *vmcontext.VMContext) iscp.Sandbox {
		return &sandbox{
			vmctx:  vmctx,
			assert: assert.NewAssert(vmctx),
		}
	}
}

func (s *sandbox) AccountID() *iscp.AgentID {
	return s.vmctx.AccountID()
}

func (s *sandbox) BalanceIotas() uint64 {
	return s.vmctx.GetIotaBalance(s.vmctx.AccountID())
}

func (s *sandbox) BalanceNativeToken(id *iotago.NativeTokenID) *big.Int {
	return s.vmctx.GetNativeTokenBalance(s.vmctx.AccountID(), id)
}

func (s *sandbox) Assets() *iscp.Assets {
	return s.vmctx.GetAssets(s.vmctx.AccountID())
}

// Call calls an entry point of contract, passes parameters and funds
func (s *sandbox) Call(target, entryPoint iscp.Hname, params dict.Dict, transfer *iscp.Assets) (dict.Dict, error) {
	return s.vmctx.Call(target, entryPoint, params, transfer)
}

func (s *sandbox) Caller() *iscp.AgentID {
	return s.vmctx.Caller()
}

func (s *sandbox) ChainID() *iscp.ChainID {
	return s.vmctx.ChainID()
}

func (s *sandbox) ChainOwnerID() *iscp.AgentID {
	return s.vmctx.ChainOwnerID()
}

func (s *sandbox) Contract() iscp.Hname {
	return s.vmctx.CurrentContractHname()
}

func (s *sandbox) ContractCreator() *iscp.AgentID {
	return s.vmctx.ContractCreator()
}

// DeployContract deploys contract by the binary hash
// and calls "init" endpoint (constructor) with provided parameters
func (s *sandbox) DeployContract(programHash hashing.HashValue, name, description string, initParams dict.Dict) error {
	err := s.vmctx.DeployContract(programHash, name, description, initParams)
	return err
}

func (s *sandbox) Event(msg string) {
	s.Log().Infof("event::%s -> '%s'", s.vmctx.CurrentContractHname(), msg)
	s.vmctx.MustSaveEvent(s.vmctx.CurrentContractHname(), msg)
}

func (s *sandbox) GetEntropy() hashing.HashValue {
	return s.vmctx.Entropy()
}

func (s *sandbox) Timestamp() int64 {
	return s.vmctx.Timestamp()
}

func (s *sandbox) AllowanceAvailable() *iscp.Assets {
	return s.vmctx.AllowanceAvailable()
}

func (s *sandbox) TransferAllowedFunds(target *iscp.AgentID, assets ...*iscp.Assets) *iscp.Assets {
	return s.vmctx.TransferAllowedFunds(target, false, assets...)
}

func (s *sandbox) TransferAllowedFundsForceCreateTarget(target *iscp.AgentID, assets ...*iscp.Assets) *iscp.Assets {
	return s.vmctx.TransferAllowedFunds(target, true, assets...)
}

func (s *sandbox) Log() iscp.LogInterface {
	// TODO should Log be disabled for wasm contracts? not much of a point in exposing internal logging
	return s.vmctx
}

func (s *sandbox) Params() dict.Dict {
	return s.vmctx.Params()
}

func (s *sandbox) Request() iscp.Calldata {
	return s.vmctx.Request()
}

func (s *sandbox) Send(par iscp.RequestParameters) {
	s.vmctx.Send(par)
}

func (s *sandbox) EstimateRequiredDustDeposit(par iscp.RequestParameters) (uint64, error) {
	return s.vmctx.EstimateRequiredDustDeposit(par)
}

func (s *sandbox) State() kv.KVStore {
	return s.vmctx.State()
}

func (s *sandbox) Utils() iscp.Utils {
	return NewUtils(s.Gas())
}

func (s *sandbox) BlockContext(construct func(ctx iscp.Sandbox) interface{}, onClose func(interface{})) interface{} {
	// doesn't have a gas burn, only used for internal (native) contracts
	return s.vmctx.BlockContext(s, construct, onClose)
}

func (s *sandbox) StateAnchor() *iscp.StateAnchor {
	return s.vmctx.StateAnchor()
}

func (s *sandbox) Gas() iscp.Gas {
	return s
}

func (s *sandbox) Burn(burnCode gas.BurnCode, par ...int) {
	s.vmctx.GasBurn(burnCode, par...)
}

func (s *sandbox) Budget() uint64 {
	return s.vmctx.GasBudgetLeft()
}

// helper methods

func (s *sandbox) Requiref(cond bool, format string, args ...interface{}) {
	s.assert.Requiref(cond, format, args...)
}

func (s *sandbox) RequireNoError(err error, str ...string) {
	s.assert.RequireNoError(err, str...)
}

func (s *sandbox) RequireCaller(agentID *iscp.AgentID, str ...string) {
	if s.Caller().Equals(agentID) {
		return
	}
	if len(str) > 0 {
		s.Log().Panicf("'%s': unauthorized access", str[0])
	} else {
		s.Log().Panicf("unauthorized access")
	}
}

func (s *sandbox) RequireCallerIsChainOwner(str ...string) {
	s.RequireCaller(s.ChainOwnerID())
}

func (s *sandbox) Privileged() iscp.Privileged {
	return s.vmctx
}
