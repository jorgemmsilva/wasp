package sbtests

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/wasp/packages/coretypes"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/vm/core"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/packages/vm/core/testcore/sbtests/sbtestsc"
	"github.com/stretchr/testify/require"
)

//nolint:revive
const (
	DEBUG        = false
	ERC20_NAME   = "erc20"
	ERC20_SUPPLY = 100000

	// ERC20 constants
	PARAM_SUPPLY  = "s"
	PARAM_CREATOR = "c"
)

var (
	WasmFileTestcore = "sbtestsc/testcore_bg.wasm"
	WasmFileErc20    = "sbtestsc/erc20_bg.wasm"
)

func setupChain(t *testing.T, keyPairOriginator *ed25519.KeyPair) (*solo.Solo, *solo.Chain) {
	core.PrintWellKnownHnames()
	env := solo.New(t, DEBUG, false)
	chain := env.NewChain(keyPairOriginator, "ch1")
	return env, chain
}

func setupDeployer(t *testing.T, chain *solo.Chain) (*ed25519.KeyPair, ledgerstate.Address, *coretypes.AgentID) {
	user, userAddr := chain.Env.NewKeyPairWithFunds()
	chain.Env.AssertAddressIotas(userAddr, solo.Saldo)

	req := solo.NewCallParams(root.Interface.Name, root.FuncGrantDeployPermission,
		root.ParamDeployer, coretypes.NewAgentID(userAddr, 0),
	).WithIotas(1)
	_, err := chain.PostRequestSync(req, nil)
	require.NoError(t, err)
	return user, userAddr, coretypes.NewAgentID(userAddr, 0)
}

func run2(t *testing.T, test func(*testing.T, bool), skipWasm ...bool) {
	t.Run(fmt.Sprintf("run CORE version of %s", t.Name()), func(t *testing.T) {
		test(t, false)
	})
	if len(skipWasm) == 0 || !skipWasm[0] {
		t.Run(fmt.Sprintf("run WASM version of %s", t.Name()), func(t *testing.T) {
			test(t, true)
		})
	} else {
		t.Logf("skipped WASM version of '%s'", t.Name())
	}
}

func setupTestSandboxSC(t *testing.T, chain *solo.Chain, user *ed25519.KeyPair, runWasm bool) (*coretypes.AgentID, uint64) {
	var err error
	var extraToken uint64
	if runWasm {
		err = chain.DeployWasmContract(user, ScName, WasmFileTestcore)
		extraToken = 1
	} else {
		err = chain.DeployContract(user, ScName, sbtestsc.Interface.ProgramHash)
		extraToken = 0
	}
	require.NoError(t, err)

	deployed := coretypes.NewAgentID(chain.ChainID.AsAddress(), HScName)
	req := solo.NewCallParams(ScName, sbtestsc.FuncDoNothing).WithIotas(1)
	_, err = chain.PostRequestSync(req, user)
	require.NoError(t, err)
	t.Logf("deployed test_sandbox'%s': %s", ScName, HScName)
	return deployed, extraToken
}

//nolint:deadcode,unused
func setupERC20(t *testing.T, chain *solo.Chain, user *ed25519.KeyPair, runWasm bool) *coretypes.AgentID {
	var err error
	if !runWasm {
		t.Logf("skipped %s. Only for Wasm tests, always loads %s", t.Name(), WasmFileErc20)
		return nil
	}
	var userAgentID *coretypes.AgentID
	if user == nil {
		userAgentID = &chain.OriginatorAgentID
	} else {
		userAddr := ledgerstate.NewED25519Address(user.PublicKey)
		userAgentID = coretypes.NewAgentID(userAddr, 0)
	}
	err = chain.DeployWasmContract(user, ERC20_NAME, WasmFileErc20,
		PARAM_SUPPLY, 1000000,
		PARAM_CREATOR, userAgentID,
	)
	require.NoError(t, err)

	deployed := coretypes.NewAgentID(chain.ChainID.AsAddress(), HScName)
	t.Logf("deployed erc20'%s': %s --  %s", ERC20_NAME, coretypes.Hn(ERC20_NAME), deployed)
	return deployed
}

func TestSetup1(t *testing.T) { run2(t, testSetup1) }
func testSetup1(t *testing.T, w bool) {
	_, chain := setupChain(t, nil)
	setupTestSandboxSC(t, chain, nil, w)
}

func TestSetup2(t *testing.T) { run2(t, testSetup2) }
func testSetup2(t *testing.T, w bool) {
	_, chain := setupChain(t, nil)
	user, _, _ := setupDeployer(t, chain)
	setupTestSandboxSC(t, chain, user, w)
}

func TestSetup3(t *testing.T) { run2(t, testSetup3) }
func testSetup3(t *testing.T, w bool) {
	_, chain := setupChain(t, nil)
	user, _, _ := setupDeployer(t, chain)
	setupTestSandboxSC(t, chain, user, w)
	// setupERC20(t, chain, user, w)
}
