package testcore

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/contracts/native/inccounter"
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/testutil/testmisc"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/corecontracts"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

func TestGovernance1(t *testing.T) {
	corecontracts.PrintWellKnownHnames()

	t.Run("empty list of allowed rotation addresses", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		chain := env.NewChain()

		lst := chain.GetAllowedStateControllerAddresses()
		require.EqualValues(t, 0, len(lst))
	})
	t.Run("add/remove allowed rotation addresses", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		chain := env.NewChain()

		_, addr1 := env.NewKeyPair()
		err := chain.AddAllowedStateController(addr1, nil)
		require.NoError(t, err)
		res := chain.GetAllowedStateControllerAddresses()
		require.EqualValues(t, 1, len(res))

		_, addr2 := env.NewKeyPair()
		err = chain.AddAllowedStateController(addr2, nil)
		require.NoError(t, err)
		res = chain.GetAllowedStateControllerAddresses()
		require.EqualValues(t, 2, len(res))

		require.True(t, addr1.Equal(res[0]) || addr1.Equal(res[1]))
		require.True(t, addr2.Equal(res[0]) || addr2.Equal(res[1]))

		err = chain.RemoveAllowedStateController(addr1, nil)
		require.NoError(t, err)
		res = chain.GetAllowedStateControllerAddresses()
		require.EqualValues(t, 1, len(res))
		require.True(t, addr2.Equal(res[0]))

		err = chain.RemoveAllowedStateController(addr1, nil)
		require.NoError(t, err)
		res = chain.GetAllowedStateControllerAddresses()
		require.EqualValues(t, 1, len(res))
		require.True(t, addr2.Equal(res[0]))

		err = chain.RemoveAllowedStateController(addr2, nil)
		require.NoError(t, err)
		res = chain.GetAllowedStateControllerAddresses()
		require.EqualValues(t, 0, len(res))
	})
}

func TestRotate(t *testing.T) {
	t.Run("not allowed address", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		chain := env.NewChain()

		kp, addr := env.NewKeyPair()
		err := chain.RotateStateController(addr, kp, nil)
		require.Error(t, err)
		strings.Contains(err.Error(), "checkRotateCommitteeRequest: address is not allowed as next state address")
	})
	t.Run("unauthorized", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		chain := env.NewChain()

		kp, addr := env.NewKeyPairWithFunds()
		err := chain.RotateStateController(addr, kp, kp)
		require.Error(t, err)
		strings.Contains(err.Error(), "checkRotateStateControllerRequest: unauthorized access")
	})
	t.Run("rotate success", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		chain := env.NewChain()

		chain.WaitUntilMempoolIsEmpty()

		newKP, newAddr := env.NewKeyPair()
		err := chain.AddAllowedStateController(newAddr, nil)
		require.NoError(t, err)

		err = chain.RotateStateController(newAddr, newKP, nil)
		require.NoError(t, err)

		chain.WaitUntilMempoolIsEmpty()

		ca := chain.GetControlAddresses()
		require.True(t, ca.StateAddress.Equal(newAddr))

		req := solo.NewCallParams(isc.NewMessageFromNames("dummy", "dummy")).
			WithMaxAffordableGasBudget()
		_, err = chain.PostRequestSync(req, nil)
		testmisc.RequireErrorToBe(t, err, vm.ErrContractNotFound)
	})

	t.Run("rotation tx at the same time as normal requests", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		chain := env.NewChain()

		chain.WaitUntilMempoolIsEmpty()

		newKP, newAddr := env.NewKeyPair()
		err := chain.AddAllowedStateController(newAddr, nil)
		require.NoError(t, err)
		someWallet, someAddr := env.NewKeyPairWithFunds()
		someAgentID := isc.NewAddressAgentID(someAddr)
		chain.DepositAssetsToL2(isc.NewAssetsBaseTokens(10*isc.Million), someWallet)

		rotationReq := solo.NewCallParams(governance.FuncRotateStateController.Message(newAddr)).
			WithMaxAffordableGasBudget().NewRequestOffLedger(chain, chain.OriginatorPrivateKey)
		dummyReq := solo.NewCallParams(isc.NewMessageFromNames("dummy", "dummy")).WithNonce(0).WithMaxAffordableGasBudget().NewRequestOffLedger(chain, someWallet)
		dummyReq2 := solo.NewCallParams(isc.NewMessageFromNames("dummy", "dummy")).WithNonce(1).WithMaxAffordableGasBudget().NewRequestOffLedger(chain, someWallet)

		previousBlock := chain.GetLatestBlockInfo()
		previousStateMetadata := chain.GetChainOutputsFromL1().AnchorOutput.FeatureSet().StateMetadata().Entries[""]
		previousAOID := chain.GetChainOutputsFromL1().AnchorOutputID
		previousBal := chain.L2BaseTokens(someAgentID)

		// asert that when sending a rotation request in the same batch as other requests, only the rotation request gets executed.
		chain.RunRequestsSync([]isc.Request{dummyReq, rotationReq, dummyReq2}, "")
		chain.StateControllerKeyPair = newKP

		// same block number
		require.Equal(t, previousBlock.BlockIndex(), chain.GetLatestBlockInfo().BlockIndex())
		// different AO
		require.NotEqual(t, previousAOID, chain.GetChainOutputsFromL1().AnchorOutputID)
		// state commitment is kept the same as previous
		require.Equal(t, previousStateMetadata, chain.GetChainOutputsFromL1().AnchorOutput.FeatureSet().StateMetadata().Entries[""])

		// assert no funds were taken from "someWallet" account, because the dummy requests were not executed
		require.EqualValues(t, previousBal, chain.L2BaseTokens(someAgentID))

		// no receipts for the dummy requests
		_, ok := chain.GetRequestReceipt(dummyReq.ID())
		require.False(t, ok)

		// if the dummy requests are sent now, they will be executed (not skipped)
		chain.RunRequestsSync([]isc.Request{dummyReq, dummyReq2}, "")
		rec, _ := chain.GetRequestReceipt(dummyReq.ID())
		require.NotNil(t, rec)
		rec, _ = chain.GetRequestReceipt(dummyReq2.ID())
		require.NotNil(t, rec)
	})
}

func TestAccessNodes(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
	node1KP, _ := env.NewKeyPairWithFunds()
	node1OwnerKP, node1OwnerAddr := env.NewKeyPairWithFunds()
	chainKP, _ := env.NewKeyPairWithFunds()
	chain, _ := env.NewChainExt(chainKP, 0, initMana, "chain1")
	var res dict.Dict
	var err error

	//
	// Initially the state is empty.
	res, err = chain.CallView(governance.ViewGetChainNodes.Message())
	require.NoError(t, err)
	getChainNodesResponse, err := governance.ViewGetChainNodes.Output.Decode(res)
	require.NoError(t, err)
	require.Empty(t, getChainNodesResponse.AccessNodeCandidates)
	require.Empty(t, getChainNodesResponse.AccessNodes)

	//
	// Add a single access node candidate.
	_, err = chain.PostRequestSync(
		solo.NewCallParams(
			governance.FuncAddCandidateNode.Message((&governance.AccessNodeInfo{
				NodePubKey:   node1KP.GetPublicKey().AsBytes(),
				ForCommittee: false,
				AccessAPI:    "http://my-api/url",
			}).AddCertificate(node1KP, node1OwnerAddr)),
		).WithMaxAffordableGasBudget(),
		node1OwnerKP, // Sender should match data used to create the Cert field value.
	)
	require.NoError(t, err)

	res, err = chain.CallView(governance.ViewGetChainNodes.Message())
	require.NoError(t, err)
	getChainNodesResponse, err = governance.ViewGetChainNodes.Output.Decode(res)
	require.NoError(t, err)
	require.Equal(t, 1, len(getChainNodesResponse.AccessNodeCandidates)) // Candidate registered.
	require.Equal(t, "http://my-api/url", getChainNodesResponse.AccessNodeCandidates[node1KP.GetPublicKey().AsKey()].AccessAPI)
	require.Empty(t, getChainNodesResponse.AccessNodes)

	//
	// Accept the node as an access node.
	_, err = chain.PostRequestSync(
		solo.NewCallParams(governance.FuncChangeAccessNodes.Message(
			governance.NewChangeAccessNodesRequest().
				Accept(node1KP.GetPublicKey()),
		)).WithMaxAffordableGasBudget(),
		chainKP,
	)
	require.NoError(t, err)

	res, err = chain.CallView(governance.ViewGetChainNodes.Message())
	require.NoError(t, err)
	getChainNodesResponse, err = governance.ViewGetChainNodes.Output.Decode(res)
	require.NoError(t, err)
	require.Equal(t, 1, len(getChainNodesResponse.AccessNodeCandidates)) // Candidate registered.
	require.Equal(t, "http://my-api/url", getChainNodesResponse.AccessNodeCandidates[node1KP.GetPublicKey().AsKey()].AccessAPI)
	require.Equal(t, 1, len(getChainNodesResponse.AccessNodes))

	//
	// Revoke the access node (by the node owner).
	_, err = chain.PostRequestSync(
		solo.NewCallParams(
			governance.FuncRevokeAccessNode.Message((&governance.AccessNodeInfo{
				NodePubKey: node1KP.GetPublicKey().AsBytes(),
			}).AddCertificate(node1KP, node1OwnerAddr)),
		).WithMaxAffordableGasBudget(),
		node1OwnerKP, // Sender should match data used to create the Cert field value.
	)
	require.NoError(t, err)

	res, err = chain.CallView(governance.ViewGetChainNodes.Message())
	require.NoError(t, err)
	getChainNodesResponse, err = governance.ViewGetChainNodes.Output.Decode(res)
	require.NoError(t, err)
	require.Empty(t, getChainNodesResponse.AccessNodeCandidates)
	require.Empty(t, getChainNodesResponse.AccessNodes)
}

func TestMaintenanceMode(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true}).
		WithNativeContract(inccounter.Processor)
	ch := env.NewChain()

	ownerWallet, ownerAddr := env.NewKeyPairWithFunds()
	ownerAgentID := isc.NewAgentID(ownerAddr)
	ch.DepositBaseTokensToL2(10*isc.Million, ownerWallet)

	userWallet, _ := env.NewKeyPairWithFunds()
	ch.DepositBaseTokensToL2(10*isc.Million, userWallet)

	// set owner of the chain
	{
		_, err2 := ch.PostRequestSync(
			solo.NewCallParams(governance.FuncDelegateChainOwnership.Message(ownerAgentID)).
				WithMaxAffordableGasBudget(),
			nil,
		)
		require.NoError(t, err2)

		_, err2 = ch.PostRequestSync(
			solo.NewCallParams(governance.FuncClaimChainOwnership.Message()).WithMaxAffordableGasBudget(),
			ownerWallet,
		)
		require.NoError(t, err2)
	}

	// call the gov "maintenance status view", check it is OFF
	{
		// TODO: Add maintenance status to wrapped core contracts
		ret, err2 := ch.CallView(governance.ViewGetMaintenanceStatus.Message())
		require.NoError(t, err2)
		maintenanceStatus := lo.Must(governance.ViewGetMaintenanceStatus.Output.Decode(ret))
		require.False(t, maintenanceStatus)
	}

	// test non-chain owner cannot call init maintenance
	{
		_, err2 := ch.PostRequestSync(
			solo.NewCallParams(governance.FuncStartMaintenance.Message()).WithMaxAffordableGasBudget(),
			userWallet,
		)
		require.ErrorContains(t, err2, "unauthorized")
	}

	// owner can start maintenance mode
	{
		_, err2 := ch.PostRequestSync(
			solo.NewCallParams(governance.FuncStartMaintenance.Message()).WithMaxAffordableGasBudget(),
			ownerWallet,
		)
		require.NoError(t, err2)
	}

	// call the gov "maintenance status view", check it is ON
	{
		ret, err2 := ch.CallView(governance.ViewGetMaintenanceStatus.Message())
		require.NoError(t, err2)
		maintenanceStatus := lo.Must(governance.ViewGetMaintenanceStatus.Output.Decode(ret))
		require.True(t, maintenanceStatus)
	}

	// calls to non-maintenance endpoints are not processed
	ch.WaitForRequestsMark()

	var reqs []isc.OffLedgerRequest
	{
		for _, wallet := range []*cryptolib.KeyPair{userWallet, ownerWallet} {
			req := solo.NewCallParams(inccounter.FuncIncCounter.Message(nil)).
				WithMaxAffordableGasBudget().
				NewRequestOffLedger(ch, wallet)
			env.AddRequestsToMempool(ch, []isc.Request{req})
			reqs = append(reqs, req)
		}
	}

	// give some time for the requests to be picked up from the mempool
	require.False(t, ch.WaitForRequestsThrough(2, 200*time.Millisecond))

	// requests are skipped
	for _, req := range reqs {
		require.False(t, ch.IsRequestProcessed(req.ID()))
	}

	fp := &gas.FeePolicy{
		GasPerToken:       util.Ratio32{A: 1, B: 10},
		ValidatorFeeShare: 1,
		EVMGasRatio:       gas.DefaultEVMGasRatio,
	}

	// calls to governance are processed (try changing fees for example)
	{
		_, err2 := ch.PostRequestSync(solo.NewCallParams(governance.FuncSetFeePolicy.Message(fp)), ownerWallet)
		require.NoError(t, err2)
	}

	// calls to governance from non-owners should be processed, but fail
	{
		_, err2 := ch.PostRequestSync(solo.NewCallParams(governance.FuncSetFeePolicy.Message(fp)), userWallet)
		require.ErrorContains(t, err2, "unauthorized")
	}

	// test non-chain owner cannot call stop maintenance
	{
		_, err2 := ch.PostRequestSync(
			solo.NewCallParams(governance.FuncStopMaintenance.Message()).WithMaxAffordableGasBudget(),
			userWallet,
		)
		require.ErrorContains(t, err2, "unauthorized")
	}

	// requests are still skipped
	for _, req := range reqs {
		require.False(t, ch.IsRequestProcessed(req.ID()))
	}

	ch.WaitForRequestsMark()

	// owner can stop maintenance mode
	{
		_, err2 := ch.PostRequestSync(
			solo.NewCallParams(governance.FuncStopMaintenance.Message()).WithMaxAffordableGasBudget(),
			ownerWallet,
		)
		require.NoError(t, err2)
	}

	// normal requests are now processed successfully (pending requests issued during maintenance should be processed now)
	require.True(t, ch.WaitForRequestsThrough(3, 1*time.Second))
	for _, req := range reqs {
		require.True(t, ch.IsRequestProcessed(req.ID()))
	}
}

var (
	ownerContract        = coreutil.NewContract("chain owner contract")
	claimOwnershipFunc   = coreutil.NewEP0(ownerContract, "claimOwnership")
	startMaintenanceFunc = coreutil.NewEP0(ownerContract, "initMaintenance")
)

func createOwnerContract(t *testing.T) (*solo.Chain, *coreutil.ContractInfo) {
	ownerContractProcessor := ownerContract.Processor(nil,
		claimOwnershipFunc.WithHandler(func(ctx isc.Sandbox) dict.Dict {
			return ctx.Call(governance.FuncClaimChainOwnership.Message(), nil)
		}),
		startMaintenanceFunc.WithHandler(func(ctx isc.Sandbox) dict.Dict {
			return ctx.Call(governance.FuncStartMaintenance.Message(), nil)
		}),
	)
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true}).
		WithNativeContract(ownerContractProcessor)
	ch := env.NewChain()

	err := ch.DeployContract(nil, ownerContract.Name, ownerContract.ProgramHash, nil)
	require.NoError(t, err)

	return ch, ownerContract
}

func TestDisallowMaintenanceDeadlock1(t *testing.T) {
	ch, ownerContract := createOwnerContract(t)

	ownerContractAgentID := isc.NewContractAgentID(ch.ChainID, ownerContract.Hname())
	userWallet, _ := ch.Env.NewKeyPairWithFunds()

	// from the initial owner - set maintenance
	_, err := ch.PostRequestSync(
		solo.NewCallParams(governance.FuncStartMaintenance.Message()).WithMaxAffordableGasBudget(),
		nil,
	)
	require.NoError(t, err)

	// set the "owner contract" as the new chain owner
	_, err = ch.PostRequestSync(
		solo.NewCallParams(governance.FuncDelegateChainOwnership.Message(ownerContractAgentID)).
			WithMaxAffordableGasBudget(),
		nil,
	)
	require.NoError(t, err)

	// the "owner contract" cannot claim ownership
	_, err = ch.PostRequestSync(
		solo.NewCallParams(claimOwnershipFunc.Message()).WithMaxAffordableGasBudget(),
		userWallet,
	)
	require.ErrorContains(t, err, "skipped")
}

func TestDisallowMaintenanceDeadlock2(t *testing.T) {
	ch, ownerContract := createOwnerContract(t)

	ownerContractAgentID := isc.NewContractAgentID(ch.ChainID, ownerContract.Hname())
	userWallet, _ := ch.Env.NewKeyPairWithFunds()

	// set the "owner contract" as the new chain owner
	_, err := ch.PostRequestSync(
		solo.NewCallParams(governance.FuncDelegateChainOwnership.Message(ownerContractAgentID)).
			WithMaxAffordableGasBudget(),
		nil,
	)
	require.NoError(t, err)

	_, err = ch.PostRequestSync(
		solo.NewCallParams(claimOwnershipFunc.Message()).WithMaxAffordableGasBudget(),
		userWallet,
	)
	require.NoError(t, err)

	// the "owner contract" is unable to start maintenance
	_, err = ch.PostRequestSync(
		solo.NewCallParams(startMaintenanceFunc.Message()).WithMaxAffordableGasBudget(),
		userWallet,
	)
	require.ErrorContains(t, err, "unauthorized")
}

func TestMetadata(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
	ch := env.NewChain()

	// deposit some extra tokens to the common account to accommodate for the SD change
	ch.SendFromL1ToL2AccountBaseTokens(10*isc.Million, 9*isc.Million, accounts.CommonAccount(), nil)

	/*
		Values with the length == 0 will reset the state value
		Values with the length > 0 will set the state value
		Nil values will be ignored and not change the state value
	*/

	testValue := "TESTYTEST"

	testMetadata := &isc.PublicChainMetadata{
		EVMJsonRPCURL:   testValue,
		EVMWebSocketURL: testValue,
		Name:            testValue,
		Description:     testValue,
		Website:         testValue,
	}

	// Set proper metadata value
	_, err := ch.PostRequestSync(
		solo.NewCallParams(governance.FuncSetMetadata.Message(nil, &testMetadata)).
			WithMaxAffordableGasBudget(),
		nil,
	)
	require.NoError(t, err)

	res, err := ch.CallView(governance.ViewGetMetadata.Message())
	require.NoError(t, err)
	resMetadata := lo.Must(governance.ViewGetMetadata.Output2.Decode(res))

	// Chain name should be equal to the configured one.
	require.Equal(t, testMetadata.Bytes(), resMetadata.Bytes())

	// Call SetMetadata without args. The metadata should be the same as it was previously configured and not be emptied.
	_, err = ch.PostRequestSync(
		solo.NewCallParams(governance.FuncSetMetadata.EntryPointInfo.Message(nil)).
			WithMaxAffordableGasBudget(),
		nil,
	)
	require.NoError(t, err)

	res, err = ch.CallView(governance.ViewGetMetadata.Message())
	require.NoError(t, err)
	resMetadata = lo.Must(governance.ViewGetMetadata.Output2.Decode(res))

	// Chain name should be equal to the configured one.
	require.Equal(t, testMetadata.Bytes(), resMetadata.Bytes())

	// Call SetMetadata with an empty arg. The metadata call should fail.
	_, err = ch.PostRequestSync(
		solo.NewCallParams(governance.FuncSetMetadata.EntryPointInfo.Message(dict.Dict{
			governance.ParamMetadata: []byte{},
		})).
			WithMaxAffordableGasBudget(),
		nil,
	)
	require.Error(t, err)

	// set invalid URL
	hugePublicURL := string(make([]byte, transaction.MaxPublicURLLength+1))
	_, err = ch.PostRequestOffLedger(
		solo.NewCallParams(governance.FuncSetMetadata.Message(&hugePublicURL, nil)).
			WithMaxAffordableGasBudget(),
		nil,
	)
	require.Error(t, err)
}

func TestL1Metadata(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
	ch := env.NewChain()

	// deposit some extra tokens to the common account to accommodate for the SD change
	ch.SendFromL1ToL2AccountBaseTokens(10*isc.Million, 9*isc.Million, accounts.CommonAccount(), nil)

	// set max valid size custom metadata
	publicURLMetadata := "https://iota.org"

	_, err := ch.PostRequestSync(
		solo.NewCallParams(governance.FuncSetMetadata.Message(&publicURLMetadata, nil)).
			WithMaxAffordableGasBudget(),
		nil,
	)
	require.NoError(t, err)

	// assert metadata is correct on view call
	res, err := ch.CallView(governance.ViewGetMetadata.Message())
	require.NoError(t, err)
	resMetadata := lo.Must(governance.ViewGetMetadata.Output1.Decode(res))
	require.Equal(t, publicURLMetadata, resMetadata)

	// assert metadata is correct on L1 anchor output
	co, err := ch.LatestChainOutputs(chaintypes.ActiveOrCommittedState)
	require.NoError(t, err)
	sm, err := transaction.StateMetadataFromAnchorOutput(co.AnchorOutput)
	require.NoError(t, err)
	require.Equal(t, publicURLMetadata, sm.PublicURL)
	require.True(t, reflect.DeepEqual(sm.GasFeePolicy, gas.DefaultFeePolicy()))

	// try changing the gas policy
	newFeePolicy := &gas.FeePolicy{
		GasPerToken: util.Ratio32{
			A: 1,
			B: 2,
		},
		EVMGasRatio: util.Ratio32{
			A: 3,
			B: 4,
		},
		ValidatorFeeShare: 5,
	}
	_, err = ch.PostRequestSync(
		solo.NewCallParams(governance.FuncSetFeePolicy.Message(newFeePolicy)).WithMaxAffordableGasBudget(),
		nil,
	)
	require.NoError(t, err)

	// assert gas policy changed on L1 metadata
	co, err = ch.LatestChainOutputs(chaintypes.ActiveOrCommittedState)
	require.NoError(t, err)
	sm, err = transaction.StateMetadataFromAnchorOutput(co.AnchorOutput)
	require.NoError(t, err)
	require.Equal(t, publicURLMetadata, sm.PublicURL)
	require.True(t, reflect.DeepEqual(sm.GasFeePolicy, newFeePolicy))
}

func TestGovernanceGasFee(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true, Debug: true})
	ch := env.NewChain()
	fp := ch.GetGasFeePolicy()
	fp.GasPerToken.A *= 1000000
	ch.SetGasFeePolicy(nil, fp)
	fp.GasPerToken.A /= 1000000
	ch.SetGasFeePolicy(nil, fp) // should not fail with "gas budget exceeded"
}

func TestGovernanceZeroGasFee(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true, Debug: true})
	ch := env.NewChain()

	user, userAddr := env.NewKeyPairWithFunds()
	userAgentID := isc.NewAgentID(userAddr)

	ch.SetGasFeePolicy(nil, &gas.FeePolicy{
		EVMGasRatio: gas.DefaultEVMGasRatio,
		GasPerToken: util.Ratio32{
			A: 0,
			B: 0,
		},
		ValidatorFeeShare: 1,
	})

	_, estimate, err := ch.EstimateGasOnLedger(solo.NewCallParams(accounts.FuncDeposit.Message()), user)
	require.NoError(t, err)
	require.Zero(t, estimate.GasFeeCharged)

	amount := 1 * isc.Million
	initialBalanceL1 := ch.Env.L1BaseTokens(userAddr)

	err = ch.DepositAssetsToL2(isc.NewAssetsBaseTokens(1*isc.Million), user)
	require.NoError(t, err)
	require.Equal(t, initialBalanceL1-amount, env.L1BaseTokens(userAddr))
	require.Equal(t, amount, ch.L2BaseTokens(userAgentID))
	require.NotZero(t, ch.LastReceipt().GasBurned)
	require.Zero(t, ch.LastReceipt().GasFeeCharged)
}

func TestGovernanceSetMustGetPayoutAgentID(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true, Debug: true})
	ch := env.NewChain()

	user, userAddr := env.NewKeyPairWithFunds()
	userAgentID := isc.NewAgentID(userAddr)

	_, err := ch.PostRequestSync(
		solo.NewCallParams(governance.FuncSetPayoutAgentID.Message(userAgentID)).
			WithMaxAffordableGasBudget(),
		nil,
	)
	require.NoError(t, err)

	retDict, err := ch.CallView(governance.ViewGetPayoutAgentID.Message())
	require.NoError(t, err)
	retAgentID, err := governance.ViewGetPayoutAgentID.Output.Decode(retDict)
	require.NoError(t, err)
	require.Equal(t, userAgentID, retAgentID)

	_, err = ch.PostRequestSync(
		solo.NewCallParams(governance.FuncSetPayoutAgentID.Message(userAgentID)).
			WithMaxAffordableGasBudget(),
		user,
	)
	require.ErrorContains(t, err, "unauthorized access")
}

func TestGovernanceSetGetMinCommonAccountBalance(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true, Debug: true})
	ch := env.NewChain()

	initRetDict, err := ch.CallView(governance.ViewGetMinCommonAccountBalance.Message())
	require.NoError(t, err)
	retMinCommonAccountBalance, err := governance.ViewGetMinCommonAccountBalance.Output.Decode(initRetDict)
	require.NoError(t, err)
	require.EqualValues(t, governance.DefaultMinBaseTokensOnCommonAccount, retMinCommonAccountBalance)

	minCommonAccountBalance := iotago.BaseToken(123456)
	_, err = ch.PostRequestSync(
		solo.NewCallParams(governance.FuncSetMinCommonAccountBalance.Message(minCommonAccountBalance)).
			WithMaxAffordableGasBudget(),
		nil,
	)
	require.NoError(t, err)

	retDict, err := ch.CallView(governance.ViewGetMinCommonAccountBalance.Message())
	require.NoError(t, err)
	retMinCommonAccountBalance, err = governance.ViewGetMinCommonAccountBalance.Output.Decode(retDict)
	require.NoError(t, err)
	require.Equal(t, minCommonAccountBalance, retMinCommonAccountBalance)
}

func TestGovCallsNoBalance(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{})
	ch := env.NewChain(false)

	// the owner can call gov funcs without funds
	_, err := ch.PostRequestOffLedger(
		solo.NewCallParams(governance.FuncStartMaintenance.Message()),
		nil,
	)
	require.NoError(t, err)
	_, err = ch.PostRequestOffLedger(
		solo.NewCallParams(governance.FuncStopMaintenance.Message()),
		nil,
	)
	require.NoError(t, err)
}

func TestGasPayout(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{
		AutoAdjustStorageDeposit: true,
		Debug:                    true,
	})
	ch := env.NewChain(false)
	user1, user1Addr := env.NewKeyPairWithFunds()
	user1AgentID := isc.NewAgentID(user1Addr)
	_, payoutAddr := env.NewKeyPairWithFunds()
	payoutAgentID := isc.NewAgentID(payoutAddr)

	// transfer some tokens from a new account (user1)
	ownerBal1 := ch.L2Assets(ch.OriginatorAgentID)
	user1Bal1 := ch.L2Assets(user1AgentID)
	transferAmt := 1 * isc.Million
	_, err := ch.PostRequestSync(
		solo.NewCallParams(accounts.FuncDeposit.Message()).AddBaseTokens(transferAmt),
		user1,
	)
	require.NoError(t, err)
	gasFees := ch.LastReceipt().GasFeeCharged

	// assert gas payout works as expected, owner gets the fees
	ownerBal2 := ch.L2Assets(ch.OriginatorAgentID)
	commonBal2 := ch.L2CommonAccountAssets()
	user1Bal2 := ch.L2Assets(user1AgentID)
	require.Equal(t, ownerBal2.BaseTokens, ownerBal1.BaseTokens+gasFees)
	require.Equal(t, user1Bal2.BaseTokens, user1Bal1.BaseTokens+transferAmt-gasFees)

	// change the payoutAddress, so that user1 now receives the fees charged by the chain
	_, err = ch.PostRequestOffLedger(
		solo.NewCallParams(governance.FuncSetPayoutAgentID.Message(payoutAgentID)),
		nil,
	)
	require.NoError(t, err)

	// no balance changes (owner calls to gov contract don't pay fees)
	ownerBal3 := ch.L2Assets(ch.OriginatorAgentID)
	commonBal3 := ch.L2CommonAccountAssets()
	user1Bal3 := ch.L2Assets(user1AgentID)
	require.Equal(t, ownerBal3.BaseTokens, ownerBal2.BaseTokens)
	require.Equal(t, commonBal3.BaseTokens, commonBal2.BaseTokens)
	require.Equal(t, user1Bal3.BaseTokens, user1Bal2.BaseTokens)

	// assert new payoutAddr is correctly set
	retDict, err := ch.CallView(governance.ViewGetPayoutAgentID.Message())
	require.NoError(t, err)
	retAgentID, err := governance.ViewGetPayoutAgentID.Output.Decode(retDict)
	require.NoError(t, err)
	require.Equal(t, payoutAgentID, retAgentID)

	// send a new request (another deposit from user1)
	_, err = ch.PostRequestSync(
		solo.NewCallParams(accounts.FuncDeposit.Message()).AddBaseTokens(transferAmt),
		user1,
	)
	require.NoError(t, err)
	gasFees = ch.LastReceipt().GasFeeCharged
	ownerBal4 := ch.L2Assets(ch.OriginatorAgentID)
	commonBal4 := ch.L2CommonAccountAssets()
	user1Bal4 := ch.L2Assets(user1AgentID)
	payoutBal4 := ch.L2Assets(payoutAgentID)
	require.Equal(t, ownerBal4.BaseTokens, ownerBal3.BaseTokens)
	require.Equal(t, commonBal4.BaseTokens, commonBal3.BaseTokens)
	require.Equal(t, user1Bal4.BaseTokens, user1Bal3.BaseTokens+transferAmt-gasFees)
	require.Equal(t, gasFees, payoutBal4.BaseTokens)
}
