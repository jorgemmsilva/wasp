package vmcontext

import (
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/subrealm"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/blob"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/errors"
	"github.com/iotaledger/wasp/packages/vm/core/evm"
	"github.com/iotaledger/wasp/packages/vm/core/evm/evmimpl"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/governance/governanceimpl"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/packages/vm/core/root/rootimpl"
)

func initStateIfNeeded(vmctx *VMContext) {
	if vmctx.task.StateDraft.BlockIndex() > 0 || vmctx.task.AnchorOutput.StateIndex > 0 {
		return
	}

	vmctx.Debugf("init chain state - begin")

	vmctx.chainState().Set(state.KeyChainID, vmctx.ChainID().Bytes())

	owner := vmctx.task.AnchorOutput.FeatureSet().SenderFeature()
	if owner == nil {
		// TODO this  be refactored. The chain owner can be == gov controller
		panic("sender feature not found in tx origin")
	}

	contractState := func(contract *coreutil.ContractInfo) kv.KVStore {
		return subrealm.New(vmctx.task.StateDraft, kv.Key(contract.Hname().Bytes()))
	}

	// TODO is this really needed?
	storageDepositAssumption := transaction.NewStorageDepositEstimate()

	// TODO how to get this?
	// blockKeepAmount, err := codec.DecodeInt32(ctx.Params().MustGet(evm.FieldBlockKeepAmount), evm.BlockKeepAmountDefault)

	// chainID := evmtypes.MustDecodeChainID(ctx.Params().MustGet(evm.FieldChainID), evm.DefaultChainID)
	evmChainID := evm.DefaultChainID
	evmBlockKeepAmount := evm.BlockKeepAmountDefault

	// init the state of each core contract
	rootimpl.SetInitialState(contractState(root.Contract))
	blob.SetInitialState(contractState(blob.Contract))
	accounts.SetInitialState(contractState(accounts.Contract), vmctx.task.AnchorOutput, vmctx.ChainID(), storageDepositAssumption)
	blocklog.SetInitialState(contractState(blocklog.Contract), vmctx.task.TimeAssumption)
	errors.SetInitialState(contractState(errors.Contract))
	governanceimpl.SetInitialState(contractState(governance.Contract), isc.NewAgentID(owner.Address))
	evmimpl.SetInitialState(contractState(evm.Contract), vmctx.task.TimeAssumption, evmChainID, evmBlockKeepAmount)

	// set block context subscriptions
	root.SubscribeBlockContext(
		contractState(root.Contract),
		evm.Contract.Hname(),
		evm.FuncOpenBlockContext.Hname(),
		evm.FuncCloseBlockContext.Hname(),
	)

	vmctx.Debugf("init chain state - success")
}
