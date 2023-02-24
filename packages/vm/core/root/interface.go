package root

import (
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv"
)

var Contract = coreutil.NewContract(coreutil.CoreContractRoot, "Root Contract")

// state variables
const (
	StateVarSchemaVersion = "v"

	StateVarContractRegistry          = "r"
	StateVarDeployPermissionsEnabled  = "a"
	StateVarDeployPermissions         = "p"
	StateVarBlockContextSubscriptions = "b"
)

// param variables
const (
	ParamDeployer                     = "dp"
	ParamHname                        = "hn"
	ParamName                         = "nm"
	ParamProgramHash                  = "ph"
	ParamContractRecData              = "dt"
	ParamContractFound                = "cf"
	ParamDescription                  = "ds"
	ParamDeployPermissionsEnabled     = "de"
	ParamStorageDepositAssumptionsBin = "db"
)

// ParamEVM allows to pass init parameters to the EVM core contract, by decorating
// them with a prefix. For example:
//
//	ParamEVM(evm.FieldBlockKeepAmount)
func ParamEVM(k kv.Key) kv.Key { return "evm" + k }

// function names
var (
	FuncDeployContract           = coreutil.Func("deployContract")
	FuncGrantDeployPermission    = coreutil.Func("grantDeployPermission")
	FuncRevokeDeployPermission   = coreutil.Func("revokeDeployPermission")
	FuncRequireDeployPermissions = coreutil.Func("requireDeployPermissions")
	ViewFindContract             = coreutil.ViewFunc("findContract")
	ViewGetContractRecords       = coreutil.ViewFunc("getContractRecords")
)
