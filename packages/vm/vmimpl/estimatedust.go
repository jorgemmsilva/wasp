package vmimpl

import (
	"github.com/ethereum/go-ethereum/common"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm/core/evm"
)

func (reqctx *requestContext) estimateRequiredStorageDeposit(par isc.RequestParameters) iotago.BaseToken {
	par.AdjustToMinimumStorageDeposit = false

	hname := reqctx.CurrentContractHname()
	contractIdentity := isc.ContractIdentityFromHname(hname)
	if hname == evm.Contract.Hname() {
		contractIdentity = isc.ContractIdentityFromEVMAddress(common.Address{}) // use empty EVM address as STUB
	}
	out := transaction.BasicOutputFromPostData(
		reqctx.vm.task.AnchorOutput.AccountID.ToAddress(),
		contractIdentity,
		par,
	)
	sd, err := parameters.Storage().MinDeposit(out)
	if err != nil {
		panic(err)
	}
	return sd
}
