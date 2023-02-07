// 'root' a core contract on the chain. It is responsible for:
// - initial setup of the chain during chain deployment
// - maintaining of core parameters of the chain
// - maintaining (setting, delegating) chain owner ID
// - maintaining (granting, revoking) smart contract deployment rights
// - deployment of smart contracts on the chain and maintenance of contract registry

package rootimpl

import (
	"fmt"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/kv/subrealm"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/blob"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/errors"
	"github.com/iotaledger/wasp/packages/vm/core/evm"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/root"
)

var Processor = root.Contract.Processor(initialize,
	root.FuncDeployContract.WithHandler(deployContract),
	root.FuncGrantDeployPermission.WithHandler(grantDeployPermission),
	root.FuncRequireDeployPermissions.WithHandler(requireDeployPermissions),
	root.FuncRevokeDeployPermission.WithHandler(revokeDeployPermission),
	root.ViewFindContract.WithHandler(findContract),
	root.ViewGetContractRecords.WithHandler(getContractRecords),
	root.FuncSubscribeBlockContext.WithHandler(subscribeBlockContext),
)

// initialize handles constructor, the "init" request. This is the first call to the chain
// if it fails, chain is not initialized. Does the following:
// - stores chain ID and chain description in the state
// - sets state ownership to the caller
// - creates record in the registry for the 'root' itself
// - deploys other core contracts: 'accounts', 'blob', 'blocklog' by creating records in the registry and calling constructors
// Input:
// - ParamChainID isc.ChainID. ID of the chain. Cannot be changed
// - ParamDescription string defaults to "N/A"
// - ParamStorageDepositAssumptionsBin encoded assumptions about minimum storage deposit for internal outputs
func initialize(ctx isc.Sandbox) []byte {
	ctx.Log().Debugf("root.initialize.begin")

	state := ctx.State()
	stateAnchor := ctx.StateAnchor()
	contractRegistry := collections.NewMap(state, root.StateVarContractRegistry)
	creator := stateAnchor.Sender

	sender := ctx.Request().SenderAccount()
	senderHname, _ := isc.HnameFromAgentID(sender)
	senderAddress, _ := isc.AddressFromAgentID(sender)
	initConditionsCorrect := stateAnchor.IsOrigin &&
		state.MustGet(root.StateVarStateInitialized) == nil &&
		senderHname == 0 &&
		creator != nil &&
		creator.Equal(senderAddress) &&
		contractRegistry.MustLen() == 0
	ctx.Requiref(initConditionsCorrect, "root.initialize.fail: %v", root.ErrChainInitConditionsFailed)

	assetsOnStateAnchor := isc.NewAssets(stateAnchor.Deposit, nil)
	ctx.Requiref(len(assetsOnStateAnchor.NativeTokens) == 0, "root.initialize.fail: native tokens in origin output are not allowed")

	// store 'root' into the registry
	storeCoreContract(ctx, root.Contract)

	// store 'blob' into the registry and run init
	storeAndInitCoreContract(ctx, blob.Contract, nil)

	// store 'accounts' into the registry and run init
	storeAndInitCoreContract(ctx, accounts.Contract, dict.Dict{
		accounts.ParamStorageDepositAssumptionsBin: ctx.Params().MustGet(root.ParamStorageDepositAssumptionsBin),
	})

	// store 'blocklog' into the registry and run init
	storeAndInitCoreContract(ctx, blocklog.Contract, nil)

	// store 'errors' into the registry and run init
	storeAndInitCoreContract(ctx, errors.Contract, nil)

	// store 'governance' into the registry and run init
	storeAndInitCoreContract(ctx, governance.Contract, dict.Dict{
		governance.ParamChainID: codec.EncodeChainID(ctx.ChainID()),
		// chain owner is whoever creates origin and sends the 'init' request
		governance.ParamChainOwner:     sender.Bytes(),
		governance.ParamDescription:    ctx.Params().MustGet(governance.ParamDescription),
		governance.ParamFeePolicyBytes: ctx.Params().MustGet(governance.ParamFeePolicyBytes),
	})

	// store 'evm' into the registry and run init
	// filter all params that have ParamEVM prefix, and remove the prefix
	evmParams, err := dict.FromKVStore(subrealm.New(ctx.Params().Dict, root.ParamEVM("")))
	ctx.RequireNoError(err)
	storeAndInitCoreContract(ctx, evm.Contract, evmParams)

	state.Set(root.StateVarDeployPermissionsEnabled, codec.EncodeBool(true))
	state.Set(root.StateVarStateInitialized, []byte{0xFF})
	// storing hname as a terminal value of the contract's state root.
	// This way we will be able to retrieve commitment to the contract's state
	ctx.State().Set("", ctx.Contract().Bytes())

	ctx.Log().Debugf("root.initialize.success")
	return nil
}

// deployContract deploys contract and calls its 'init' constructor.
// If call to the constructor returns an error or an other error occurs,
// removes smart contract form the registry as if it was never attempted to deploy
// Inputs:
//   - ParamName string, the unique name of the contract in the chain. Later used as hname
//   - ParamProgramHash HashValue is a hash of the blob which represents program binary in the 'blob' contract.
//     In case of hardcoded examples its an arbitrary unique hash set in the global call examples.AddProcessor
//   - ParamDescription string is an arbitrary string. Defaults to "N/A"
func deployContract(ctx isc.Sandbox) []byte {
	ctx.Log().Debugf("root.deployContract.begin")
	ctx.Requiref(isAuthorizedToDeploy(ctx), "root.deployContract: deploy not permitted for: %s", ctx.Caller().String())

	progHash := ctx.Params().MustGetHashValue(root.ParamProgramHash)
	description := ctx.Params().MustGetString(root.ParamDescription, "N/A")
	name := ctx.Params().MustGetString(root.ParamName)
	ctx.Requiref(name != "", "wrong name")

	// pass to init function all params not consumed so far
	initParams := dict.New()
	err := ctx.Params().Dict.Iterate("", func(key kv.Key, value []byte) bool {
		if key != root.ParamProgramHash && key != root.ParamName && key != root.ParamDescription {
			initParams.Set(key, value)
		}
		return true
	})
	ctx.RequireNoError(err)
	// call to load VM from binary to check if it loads successfully
	err = ctx.Privileged().TryLoadContract(progHash)
	ctx.RequireNoError(err, "root.deployContract.fail 1: ")

	// VM loaded successfully. Storing contract in the registry and calling constructor
	storeContractRecord(ctx, &root.ContractRecord{
		ProgramHash: progHash,
		Description: description,
		Name:        name,
	})
	ctx.Call(isc.Hn(name), isc.EntryPointInit, initParams, nil)
	ctx.Event(fmt.Sprintf("[deploy] name: %s hname: %s, progHash: %s, dscr: '%s'",
		name, isc.Hn(name), progHash.String(), description))
	return nil
}

// grantDeployPermission grants permission to deploy contracts
// Input:
//   - ParamDeployer isc.AgentID
func grantDeployPermission(ctx isc.Sandbox) []byte {
	ctx.RequireCallerIsChainOwner()

	deployer := ctx.Params().MustGetAgentID(root.ParamDeployer)
	ctx.Requiref(deployer.Kind() != isc.AgentIDKindNil, "cannot grant deploy permission to NilAgentID")

	collections.NewMap(ctx.State(), root.StateVarDeployPermissions).MustSetAt(deployer.Bytes(), []byte{0xFF})
	ctx.Event(fmt.Sprintf("[grant deploy permission] to agentID: %s", deployer.String()))
	return nil
}

// revokeDeployPermission revokes permission to deploy contracts
// Input:
//   - ParamDeployer isc.AgentID
func revokeDeployPermission(ctx isc.Sandbox) []byte {
	ctx.RequireCallerIsChainOwner()

	deployer := ctx.Params().MustGetAgentID(root.ParamDeployer)

	collections.NewMap(ctx.State(), root.StateVarDeployPermissions).MustDelAt(deployer.Bytes())
	ctx.Event(fmt.Sprintf("[revoke deploy permission] from agentID: %v", deployer))
	return nil
}

func requireDeployPermissions(ctx isc.Sandbox) []byte {
	ctx.RequireCallerIsChainOwner()
	permissionsEnabled := ctx.Params().MustGetBool(root.ParamDeployPermissionsEnabled)
	ctx.State().Set(root.StateVarDeployPermissionsEnabled, codec.EncodeBool(permissionsEnabled))
	return nil
}

// findContract view finds and returns encoded record of the contract
// Input:
// - ParamHname
// Output:
// - ParamData
func findContract(ctx isc.SandboxView) []byte {
	hname := ctx.Params().MustGetHname(root.ParamHname)
	rec := root.FindContract(ctx.StateR(), hname)
	if rec != nil {
		return nil
	}
	return rec.Bytes()
}

func getContractRecords(ctx isc.SandboxView) []byte {
	src := root.GetContractRegistryR(ctx.StateR())

	ret := make(map[isc.Hname][]byte)
	src.MustIterate(func(elemKey []byte, value []byte) bool {
		hname, err := isc.HnameFromBytes(elemKey)
		ctx.RequireNoError(err)
		ret[hname] = value
		return true
	})

	return util.MustSerialize(ret)
}

func subscribeBlockContext(ctx isc.Sandbox) []byte {
	ctx.Requiref(ctx.StateAnchor().StateIndex == 0, "subscribeBlockContext must be called when initializing the chain")
	root.SubscribeBlockContext(
		ctx.State(),
		ctx.Caller().(*isc.ContractAgentID).Hname(),
		ctx.Params().MustGetHname(root.ParamBlockContextOpenFunc),
		ctx.Params().MustGetHname(root.ParamBlockContextCloseFunc),
	)
	return nil
}
