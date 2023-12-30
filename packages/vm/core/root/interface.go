package root

import (
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/kv/dict"
)

var Contract = coreutil.NewContract(coreutil.CoreContractRoot)

var (
	// Funcs
	FuncDeployContract        = EPDeployContract{EntryPointInfo: Contract.Func("deployContract")}
	FuncGrantDeployPermission = coreutil.NewEP1(Contract, "grantDeployPermission",
		ParamDeployer, codec.AgentID,
	)
	FuncRevokeDeployPermission = coreutil.NewEP1(Contract, "revokeDeployPermission",
		ParamDeployer, codec.AgentID,
	)
	FuncRequireDeployPermissions = coreutil.NewEP1(Contract, "requireDeployPermissions",
		ParamDeployPermissionsEnabled, codec.Bool,
	)

	// Views
	ViewFindContract = coreutil.NewViewEP12(Contract, "findContract",
		ParamHname, codec.Hname,
		ParamContractFound, codec.Bool,
		ParamContractRecData, codec.NewCodecEx(ContractRecordFromBytes),
	)
	ViewGetContractRecords = EPGetContractRecords{EP0: coreutil.NewViewEP0(Contract, "getContractRecords")}
)

// state variables
const (
	VarSchemaVersion            = "v"
	VarContractRegistry         = "r"
	VarDeployPermissionsEnabled = "a"
	VarDeployPermissions        = "p"
)

// request parameters
const (
	ParamDeployer                 = "dp"
	ParamHname                    = "hn"
	ParamName                     = "nm"
	ParamProgramHash              = "ph"
	ParamContractRecData          = "dt"
	ParamContractFound            = "cf"
	ParamDeployPermissionsEnabled = "de"
)

type EPDeployContract struct {
	coreutil.EntryPointInfo[isc.Sandbox]
}

func (e EPDeployContract) Message(name string, programHash hashing.HashValue, initParams dict.Dict) isc.Message {
	d := initParams.Clone()
	d[ParamProgramHash] = codec.HashValue.Encode(programHash)
	d[ParamName] = codec.String.Encode(name)
	return e.EntryPointInfo.Message(d)
}

type EPGetContractRecords struct {
	coreutil.EP0[isc.SandboxView]
	Output ContractRecordsOutput
}

type ContractRecordsOutput struct{}

func (c ContractRecordsOutput) Decode(d dict.Dict) (map[isc.Hname]*ContractRecord, error) {
	return decodeContractRegistry(collections.NewMapReadOnly(d, VarContractRegistry))
}
