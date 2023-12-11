package testcore

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/testutil/testdbhash"
	"github.com/iotaledger/wasp/packages/testutil/testmisc"
	"github.com/iotaledger/wasp/packages/testutil/utxodb"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/blob"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/packages/vm/core/testcore/sbtests/sbtestsc"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

func GetStorageDeposit(tx *iotago.Transaction) []iotago.BaseToken {
	ret := make([]iotago.BaseToken, len(tx.Outputs))
	for i, out := range tx.Outputs {
		ret[i] = lo.Must(testutil.L1API.StorageScoreStructure().MinDeposit(out))
	}
	return ret
}

func TestInitLoad(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
	user, userAddr := env.NewKeyPairWithFunds(env.NewSeedFromIndex(12))
	env.AssertL1BaseTokens(userAddr, utxodb.FundsFromFaucetAmount)
	originAmount := 10 * isc.Million
	ch, _ := env.NewChainExt(user, originAmount, initMana, "chain1")
	_ = ch.Log().Sync()

	cassets := ch.L2CommonAccountAssets()
	require.EqualValues(t,
		originAmount-lo.Must(testutil.L1API.StorageScoreStructure().MinDeposit(ch.GetChainOutputsFromL1().AnchorOutput)),
		cassets.BaseTokens)
	require.EqualValues(t, 0, len(cassets.NativeTokens))

	t.Logf("common base tokens: %d", ch.L2CommonAccountBaseTokens())
	require.True(t, cassets.BaseTokens >= governance.DefaultMinBaseTokensOnCommonAccount)

	testdbhash.VerifyDBHash(env, t.Name())
}

// TestLedgerBaseConsistency deploys chain and check consistency of L1 and L2 ledgers
func TestLedgerBaseConsistency(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true, Debug: true})
	genesisAddr := env.L1Ledger().GenesisAddress()
	assets := env.L1Assets(genesisAddr)
	require.EqualValues(t, env.L1Ledger().Supply(), assets.BaseTokens)

	// create chain
	ch, _ := env.NewChainExt(nil, 10*isc.Million, initMana, "chain1")
	defer func() {
		_ = ch.Log().Sync()
	}()

	// get all native tokens. Must be empty
	nativeTokenIDs := ch.GetOnChainTokenIDs()
	require.EqualValues(t, 0, len(nativeTokenIDs))

	// all goes to storage deposit and to total base tokens on chain
	// what has left on L1 address
	env.AssertL1BaseTokens(ch.OriginatorAddress, utxodb.FundsFromFaucetAmount-10*isc.Million)

	// check if there's a single anchor output on chain's address
	chainOutputs := ch.GetChainOutputsFromL1()

	// check total on chain assets
	totalL2Assets := ch.L2TotalAssets()
	// no native tokens expected
	require.EqualValues(t, 0, len(totalL2Assets.NativeTokens))

	// what spent all goes to the anchor output
	// require.EqualValues(t, int(totalSpent), int(aliasOut.Amount))
	// total base tokens on L2 must be equal to anchor output base tokens - storage deposit
	anchorSD := lo.Must(testutil.L1API.StorageScoreStructure().MinDeposit(chainOutputs.AnchorOutput))
	require.False(t, chainOutputs.HasAccountOutput())

	ch.AssertL2TotalBaseTokens(chainOutputs.AnchorOutput.Amount - anchorSD)

	// common account is empty

	someUserWallet, _ := env.NewKeyPairWithFunds()
	ch.DepositBaseTokensToL2(1*isc.Million, someUserWallet)

	chainOutputs = ch.GetChainOutputsFromL1()
	anchorSD = lo.Must(testutil.L1API.StorageScoreStructure().MinDeposit(chainOutputs.AnchorOutput))
	accountSD := lo.Must(testutil.L1API.StorageScoreStructure().MinDeposit(chainOutputs.MustAccountOutput()))
	require.EqualValues(t, accountSD, chainOutputs.MustAccountOutput().BaseTokenAmount())

	ch.AssertL2TotalBaseTokens(chainOutputs.AnchorOutput.Amount - anchorSD)
	ch.AssertControlAddresses()
}

// TestNoTargetPostOnLedger test what happens when sending requests to non-existent contract or entry point
func TestNoTargetPostOnLedger(t *testing.T) {
	t.Run("no contract,originator==user", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		ch, _ := env.NewChainExt(nil, 0, initMana, "chain")
		oldSD := ch.GetChainOutputsFromL1().StorageDeposit(testutil.L1APIProvider)

		totalBaseTokensBefore := ch.L2TotalBaseTokens()
		originatorsL2BaseTokensBefore := ch.L2BaseTokens(ch.OriginatorAgentID)
		originatorsL1BaseTokensBefore := env.L1BaseTokens(ch.OriginatorAddress)
		commonAccountBaseTokensBefore := ch.L2CommonAccountBaseTokens()
		require.GreaterOrEqual(t, commonAccountBaseTokensBefore, governance.DefaultMinBaseTokensOnCommonAccount)

		req := solo.NewCallParams("dummyContract", "dummyEP").
			WithGasBudget(100_000)
		reqTx, _, err := ch.PostRequestSyncTx(req, nil)
		// expecting specific error
		require.Contains(t, err.Error(), vm.ErrContractNotFound.Create(isc.Hn("dummyContract")).Error())

		totalBaseTokensAfter := ch.L2TotalBaseTokens()
		commonAccountBaseTokensAfter := ch.L2CommonAccountBaseTokens()

		// AO minCommonAccountBalance changes from block 0 to block 1 because the statemedata grows
		newSD := ch.GetChainOutputsFromL1().StorageDeposit(testutil.L1APIProvider)
		require.Greater(t, newSD, oldSD)
		changeInAOminCommonAccountBalance := newSD - oldSD

		reqStorageDeposit := GetStorageDeposit(reqTx.Transaction)[0]

		// total base tokens on chain increase by the storage deposit from the request tx
		require.EqualValues(t, int(totalBaseTokensBefore+reqStorageDeposit-changeInAOminCommonAccountBalance), int(totalBaseTokensAfter))
		// user on L1 is charged with storage deposit
		env.AssertL1BaseTokens(ch.OriginatorAddress, originatorsL1BaseTokensBefore-reqStorageDeposit)
		// originator (user) is charged with gas fee on L2
		ch.AssertL2BaseTokens(ch.OriginatorAgentID, originatorsL2BaseTokensBefore+reqStorageDeposit)
		// all gas fee goes to the common account
		require.EqualValues(t, commonAccountBaseTokensBefore-changeInAOminCommonAccountBalance, commonAccountBaseTokensAfter)
	})
	t.Run("no contract,originator!=user", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		ch, _ := env.NewChainExt(nil, 0, initMana, "chain")
		oldSD := ch.GetChainOutputsFromL1().StorageDeposit(testutil.L1APIProvider)

		senderKeyPair, senderAddr := env.NewKeyPairWithFunds(env.NewSeedFromIndex(10))
		senderAgentID := isc.NewAgentID(senderAddr)

		totalBaseTokensBefore := ch.L2TotalBaseTokens()
		originatorsL2BaseTokensBefore := ch.L2BaseTokens(ch.OriginatorAgentID)
		originatorsL1BaseTokensBefore := env.L1BaseTokens(ch.OriginatorAddress)
		env.AssertL1BaseTokens(senderAddr, utxodb.FundsFromFaucetAmount)
		commonAccountBaseTokensBefore := ch.L2CommonAccountBaseTokens()
		require.GreaterOrEqual(t, commonAccountBaseTokensBefore, governance.DefaultMinBaseTokensOnCommonAccount)

		req := solo.NewCallParams("dummyContract", "dummyEP").
			WithGasBudget(100_000)
		reqTx, _, err := ch.PostRequestSyncTx(req, senderKeyPair)
		// expecting specific error
		require.Contains(t, err.Error(), vm.ErrContractNotFound.Create(isc.Hn("dummyContract")).Error())

		totalBaseTokensAfter := ch.L2TotalBaseTokens()
		commonAccountBaseTokensAfter := ch.L2CommonAccountBaseTokens()

		// AO minCommonAccountBalance changes from block 0 to block 1 because the statemedata grows
		newSD := ch.GetChainOutputsFromL1().StorageDeposit(testutil.L1APIProvider)
		require.Greater(t, newSD, oldSD)
		changeInAOminCommonAccountBalance := newSD - oldSD

		reqStorageDeposit := GetStorageDeposit(reqTx.Transaction)[0]
		rec := ch.LastReceipt()

		// total base tokens on chain increase by the storage deposit from the request tx
		require.EqualValues(t, int(totalBaseTokensBefore+reqStorageDeposit-changeInAOminCommonAccountBalance), int(totalBaseTokensAfter))
		// originator on L1 does not change
		env.AssertL1BaseTokens(ch.OriginatorAddress, originatorsL1BaseTokensBefore)
		// user on L1 is charged with storage deposit
		env.AssertL1BaseTokens(senderAddr, utxodb.FundsFromFaucetAmount-reqStorageDeposit)
		// originator account does not change
		ch.AssertL2BaseTokens(ch.OriginatorAgentID, originatorsL2BaseTokensBefore+rec.GasFeeCharged)
		// user is charged with gas fee on L2
		ch.AssertL2BaseTokens(senderAgentID, reqStorageDeposit-rec.GasFeeCharged)
		// all gas fee goes to the common account
		require.EqualValues(t, commonAccountBaseTokensBefore-changeInAOminCommonAccountBalance, commonAccountBaseTokensAfter)
	})
	t.Run("no EP,originator==user", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		ch, _ := env.NewChainExt(nil, 0, initMana, "chain")
		oldSD := ch.GetChainOutputsFromL1().StorageDeposit(testutil.L1APIProvider)

		totalBaseTokensBefore := ch.L2TotalBaseTokens()
		originatorsL2BaseTokensBefore := ch.L2BaseTokens(ch.OriginatorAgentID)
		originatorsL1BaseTokensBefore := env.L1BaseTokens(ch.OriginatorAddress)
		commonAccountBaseTokensBefore := ch.L2CommonAccountBaseTokens()
		require.GreaterOrEqual(t, commonAccountBaseTokensBefore, governance.DefaultMinBaseTokensOnCommonAccount)

		req := solo.NewCallParams(root.Contract.Name, "dummyEP").
			WithGasBudget(100_000)
		reqTx, _, err := ch.PostRequestSyncTx(req, nil)
		// expecting specific error
		require.Contains(t, err.Error(), vm.ErrTargetEntryPointNotFound.Error())

		totalBaseTokensAfter := ch.L2TotalBaseTokens()
		commonAccountBaseTokensAfter := ch.L2CommonAccountBaseTokens()

		reqStorageDeposit := GetStorageDeposit(reqTx.Transaction)[0]

		// AO minCommonAccountBalance changes from block 0 to block 1 because the statemedata grows
		newSD := ch.GetChainOutputsFromL1().StorageDeposit(testutil.L1APIProvider)
		require.Greater(t, newSD, oldSD)
		changeInAOminCommonAccountBalance := newSD - oldSD

		// total base tokens on chain increase by the storage deposit from the request tx
		require.EqualValues(t, int(totalBaseTokensBefore+reqStorageDeposit-changeInAOminCommonAccountBalance), int(totalBaseTokensAfter))
		// user on L1 is charged with storage deposit
		env.AssertL1BaseTokens(ch.OriginatorAddress, originatorsL1BaseTokensBefore-reqStorageDeposit)
		// originator (user) is charged with gas fee on L2
		ch.AssertL2BaseTokens(ch.OriginatorAgentID, originatorsL2BaseTokensBefore+reqStorageDeposit)
		// all gas fee goes to the common account
		require.EqualValues(t, commonAccountBaseTokensBefore-changeInAOminCommonAccountBalance, commonAccountBaseTokensAfter)
	})
	t.Run("no EP,originator!=user", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		ch, _ := env.NewChainExt(nil, 0, initMana, "chain")
		oldSD := ch.GetChainOutputsFromL1().StorageDeposit(testutil.L1APIProvider)

		senderKeyPair, senderAddr := env.NewKeyPairWithFunds(env.NewSeedFromIndex(10))
		senderAgentID := isc.NewAgentID(senderAddr)

		totalBaseTokensBefore := ch.L2TotalBaseTokens()
		originatorsL2BaseTokensBefore := ch.L2BaseTokens(ch.OriginatorAgentID)
		originatorsL1BaseTokensBefore := env.L1BaseTokens(ch.OriginatorAddress)
		env.AssertL1BaseTokens(senderAddr, utxodb.FundsFromFaucetAmount)
		commonAccountBaseTokensBefore := ch.L2CommonAccountBaseTokens()
		require.GreaterOrEqual(t, commonAccountBaseTokensBefore, governance.DefaultMinBaseTokensOnCommonAccount)

		req := solo.NewCallParams(root.Contract.Name, "dummyEP").
			WithGasBudget(100_000)
		reqTx, _, err := ch.PostRequestSyncTx(req, senderKeyPair)
		// expecting specific error
		require.Contains(t, err.Error(), vm.ErrTargetEntryPointNotFound.Error())

		totalBaseTokensAfter := ch.L2TotalBaseTokens()
		commonAccountBaseTokensAfter := ch.L2CommonAccountBaseTokens()

		// AO minCommonAccountBalance changes from block 0 to block 1 because the statemedata grows
		newSD := ch.GetChainOutputsFromL1().StorageDeposit(testutil.L1APIProvider)
		require.Greater(t, newSD, oldSD)
		changeInAOminCommonAccountBalance := newSD - oldSD

		reqStorageDeposit := GetStorageDeposit(reqTx.Transaction)[0]
		rec := ch.LastReceipt()
		// total base tokens on chain increase by the storage deposit from the request tx
		require.EqualValues(t, int(totalBaseTokensBefore+reqStorageDeposit-changeInAOminCommonAccountBalance), int(totalBaseTokensAfter))
		// originator on L1 does not change
		env.AssertL1BaseTokens(ch.OriginatorAddress, originatorsL1BaseTokensBefore)
		// user on L1 is charged with storage deposit
		env.AssertL1BaseTokens(senderAddr, utxodb.FundsFromFaucetAmount-reqStorageDeposit)
		// originator account does not change
		ch.AssertL2BaseTokens(ch.OriginatorAgentID, originatorsL2BaseTokensBefore+rec.GasFeeCharged)
		// user is charged with gas fee on L2
		ch.AssertL2BaseTokens(senderAgentID, reqStorageDeposit-rec.GasFeeCharged)
		// all gas fee goes to the common account
		require.EqualValues(t, commonAccountBaseTokensBefore-changeInAOminCommonAccountBalance, commonAccountBaseTokensAfter)
	})
}

func TestNoTargetView(t *testing.T) {
	t.Run("no contract view", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		chain := env.NewChain()
		chain.AssertControlAddresses()

		_, err := chain.CallView("dummyContract", "dummyEP")
		require.Error(t, err)
	})
	t.Run("no EP view", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		chain := env.NewChain()
		chain.AssertControlAddresses()

		_, err := chain.CallView(root.Contract.Name, "dummyEP")
		require.Error(t, err)
	})
}

func TestEstimateGas(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true}).
		WithNativeContract(sbtestsc.Processor)
	ch := env.NewChain()
	ch.MustDepositBaseTokensToL2(10000, nil)
	err := ch.DeployContract(nil, sbtestsc.Contract.Name, sbtestsc.Contract.ProgramHash)
	require.NoError(t, err)

	callParams := func() *solo.CallParams {
		return solo.NewCallParams(sbtestsc.Contract.Name, sbtestsc.FuncCalcFibonacciIndirectStoreValue.Name,
			sbtestsc.ParamN, uint64(10),
		)
	}

	getResult := func() int64 {
		res, err2 := ch.CallView(sbtestsc.Contract.Name, sbtestsc.FuncViewCalcFibonacciResult.Name)
		require.NoError(t, err2)
		n, err2 := codec.DecodeInt64(res.Get(sbtestsc.ParamN), 0)
		require.NoError(t, err2)
		return n
	}

	var estimatedGas gas.GasUnits
	var estimatedGasFee iotago.BaseToken
	{
		keyPair, _ := env.NewKeyPairWithFunds()

		// we can call EstimateGas even with 0 base tokens in L2 account
		estimatedGas, estimatedGasFee, err = ch.EstimateGasOffLedger(callParams(), keyPair, true)
		require.NoError(t, err)
		require.NotZero(t, estimatedGas)
		require.NotZero(t, estimatedGasFee)
		t.Logf("estimatedGas: %d, estimatedGasFee: %d", estimatedGas, estimatedGasFee)

		// test that EstimateGas did not actually commit changes in the state
		require.EqualValues(t, 0, getResult())
	}

	for _, testCase := range []struct {
		Desc          string
		L2Balance     iotago.BaseToken
		GasBudget     gas.GasUnits
		ExpectedError string
	}{
		{
			Desc:          "0 base tokens in L2 balance to cover gas fee",
			L2Balance:     0,
			GasBudget:     estimatedGas,
			ExpectedError: "gas budget exceeded",
		},
		{
			Desc:          "insufficient base tokens in L2 balance to cover gas fee",
			L2Balance:     estimatedGasFee - 1,
			GasBudget:     estimatedGas,
			ExpectedError: "gas budget exceeded",
		},
		{
			Desc:          "insufficient gas budget",
			L2Balance:     estimatedGasFee,
			GasBudget:     estimatedGas - 1,
			ExpectedError: "gas budget exceeded",
		},
		{
			Desc:      "success",
			L2Balance: estimatedGasFee,
			GasBudget: estimatedGas,
		},
	} {
		t.Run(testCase.Desc, func(t *testing.T) {
			keyPair, addr := env.NewKeyPairWithFunds()
			agentID := isc.NewAgentID(addr)

			if testCase.L2Balance > 0 {
				// deposit must come from another user so that we have exactly the funds we need on the test account (can't send lower than storage deposit)
				anotherKeyPair, _ := env.NewKeyPairWithFunds()
				err = ch.TransferAllowanceTo(
					isc.NewAssetsBaseTokens(testCase.L2Balance),
					isc.NewAgentID(addr),
					anotherKeyPair,
				)
				require.NoError(t, err)
				balance := ch.L2BaseTokens(agentID)
				require.Equal(t, testCase.L2Balance, balance)
			}

			_, err := ch.PostRequestOffLedger(
				callParams().WithGasBudget(testCase.GasBudget),
				keyPair,
			)
			rec := ch.LastReceipt()
			fmt.Println(rec)
			if testCase.ExpectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), testCase.ExpectedError)
			} else {
				require.NoError(t, err)
				// changes committed to the state
				require.NotZero(t, getResult())
			}
		})
	}
}

func TestRepeatInit(t *testing.T) {
	t.Run("root", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		ch := env.NewChain()
		err := ch.DepositBaseTokensToL2(10_000, nil)
		require.NoError(t, err)
		req := solo.NewCallParams(root.Contract.Name, "init").
			WithGasBudget(100_000)
		_, err = ch.PostRequestSync(req, nil)
		require.Error(t, err)
		testmisc.RequireErrorToBe(t, err, vm.ErrRepeatingInitCall)
		ch.CheckAccountLedger()
	})
	t.Run("accounts", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		ch := env.NewChain()
		err := ch.DepositBaseTokensToL2(10_000, nil)
		require.NoError(t, err)
		req := solo.NewCallParams(accounts.Contract.Name, "init").
			WithGasBudget(100_000)
		_, err = ch.PostRequestSync(req, nil)
		require.Error(t, err)
		testmisc.RequireErrorToBe(t, err, vm.ErrRepeatingInitCall)
		ch.CheckAccountLedger()
	})
	t.Run("blocklog", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		ch := env.NewChain()
		err := ch.DepositBaseTokensToL2(10_000, nil)
		require.NoError(t, err)
		req := solo.NewCallParams(blocklog.Contract.Name, "init").
			WithGasBudget(100_000)
		_, err = ch.PostRequestSync(req, nil)
		require.Error(t, err)
		testmisc.RequireErrorToBe(t, err, vm.ErrRepeatingInitCall)
		ch.CheckAccountLedger()
	})
	t.Run("blob", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		ch := env.NewChain()
		err := ch.DepositBaseTokensToL2(10_000, nil)
		require.NoError(t, err)
		req := solo.NewCallParams(blob.Contract.Name, "init").
			WithGasBudget(100_000)
		_, err = ch.PostRequestSync(req, nil)
		require.Error(t, err)
		testmisc.RequireErrorToBe(t, err, vm.ErrRepeatingInitCall)
		ch.CheckAccountLedger()
	})
	t.Run("governance", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		ch := env.NewChain()
		err := ch.DepositBaseTokensToL2(10_000, nil)
		require.NoError(t, err)
		req := solo.NewCallParams(governance.Contract.Name, "init").
			WithGasBudget(100_000)
		_, err = ch.PostRequestSync(req, nil)
		require.Error(t, err)
		testmisc.RequireErrorToBe(t, err, vm.ErrRepeatingInitCall)
		ch.CheckAccountLedger()
	})
}

func TestDeployNativeContract(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true}).
		WithNativeContract(sbtestsc.Processor)

	ch := env.NewChain()

	senderKeyPair, senderAddr := env.NewKeyPairWithFunds(env.NewSeedFromIndex(10))

	err := ch.DepositBaseTokensToL2(10_000, senderKeyPair)
	require.NoError(t, err)

	// get more base tokens for originator
	originatorBalance := env.L1Assets(ch.OriginatorAddress).BaseTokens
	_, err = env.L1Ledger().GetFundsFromFaucet(ch.OriginatorAddress)
	require.NoError(t, err)
	env.AssertL1BaseTokens(ch.OriginatorAddress, originatorBalance+utxodb.FundsFromFaucetAmount)

	req := solo.NewCallParams(root.Contract.Name, root.FuncGrantDeployPermission.Name,
		root.ParamDeployer, isc.NewAgentID(senderAddr)).
		AddBaseTokens(100_000).
		WithGasBudget(100_000)
	_, err = ch.PostRequestSync(req, nil)
	require.NoError(t, err)

	err = ch.DeployContract(senderKeyPair, "sctest", sbtestsc.Contract.ProgramHash)
	require.NoError(t, err)
}

func TestFeeBasic(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
	chain := env.NewChain()
	feePolicy := chain.GetGasFeePolicy()
	require.EqualValues(t, 0, feePolicy.ValidatorFeeShare)
}

func TestBurnLog(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
	ch := env.NewChain()

	ch.MustDepositBaseTokensToL2(30_000, nil)
	rec := ch.LastReceipt()
	t.Logf("receipt 1:\n%s", rec)
	t.Logf("burn log 1:\n%s", rec.GasBurnLog)

	_, err := ch.UploadBlob(nil, "field", strings.Repeat("dummy data", 1000))
	require.NoError(t, err)

	rec = ch.LastReceipt()
	t.Logf("receipt 2:\n%s", rec)
	t.Logf("burn log 2:\n%s", rec.GasBurnLog)
}

func TestMessageSize(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{
		AutoAdjustStorageDeposit: true,
		Debug:                    true,
		PrintStackTrace:          true,
	}).
		WithNativeContract(sbtestsc.Processor)
	ch := env.NewChain()

	ch.MustDepositBaseTokensToL2(10000, nil)

	err := ch.DeployContract(nil, sbtestsc.Contract.Name, sbtestsc.Contract.ProgramHash)
	require.NoError(t, err)

	initialBlockIndex := ch.GetLatestBlockInfo().BlockIndex()

	reqSize := 5_000 // bytes
	storageDeposit := 1 * isc.Million

	maxRequestsPerBlock := iotago.MaxPayloadSize / reqSize

	reqs := make([]isc.Request, maxRequestsPerBlock+1)
	for i := 0; i < len(reqs); i++ {
		req, err := solo.ISCRequestFromCallParams(
			ch,
			solo.NewCallParams(sbtestsc.Contract.Name, sbtestsc.FuncSendLargeRequest.Name,
				sbtestsc.ParamSize, uint32(reqSize),
			).
				AddBaseTokens(storageDeposit).
				AddAllowanceBaseTokens(storageDeposit).
				WithMaxAffordableGasBudget(),
			nil,
		)
		require.NoError(t, err)
		reqs[i] = req
	}

	env.AddRequestsToMempool(ch, reqs)
	ch.WaitUntilMempoolIsEmpty()

	// request outputs are so large that they have to be processed in two separate blocks
	require.Equal(t, initialBlockIndex+2, ch.GetLatestBlockInfo().BlockIndex())

	for _, req := range reqs {
		receipt, err := ch.GetRequestReceipt(req.ID())
		require.NoError(t, err)
		require.Nil(t, receipt.Error)
	}
}

func TestInvalidSignatureRequestsAreNotProcessed(t *testing.T) {
	env := solo.New(t)
	ch := env.NewChain()
	req := isc.NewOffLedgerRequest(ch.ID(), isc.Hn("contract"), isc.Hn("entrypoint"), nil, 0, math.MaxUint64)
	badReqBytes := req.(*isc.OffLedgerRequestData).EssenceBytes()
	// append 33 bytes to the req essence to simulate a bad signature (32 bytes for the pubkey + 1 for 0 length signature)
	for i := 0; i < 33; i++ {
		badReqBytes = append(badReqBytes, 0x00)
	}
	badReq, err := isc.RequestFromBytes(badReqBytes)
	require.NoError(t, err)
	env.AddRequestsToMempool(ch, []isc.Request{badReq})
	time.Sleep(200 * time.Millisecond)
	// request won't be processed
	receipt, err := ch.GetRequestReceipt(badReq.ID())
	require.NoError(t, err)
	require.Nil(t, receipt)
}

func TestBatchWithSkippedRequestsReceipts(t *testing.T) {
	env := solo.New(t)
	ch := env.NewChain()
	user, _ := env.NewKeyPairWithFunds()
	err := ch.DepositAssetsToL2(isc.NewAssetsBaseTokens(10*isc.Million), user)
	require.NoError(t, err)

	// create a request with an invalid nonce that must be skipped
	skipReq := isc.NewOffLedgerRequest(ch.ID(), isc.Hn("contract"), isc.Hn("entrypoint"), nil, 0, math.MaxUint64).WithNonce(9999).Sign(user)
	validReq := isc.NewOffLedgerRequest(ch.ID(), isc.Hn("contract"), isc.Hn("entrypoint"), nil, 0, math.MaxUint64).WithNonce(0).Sign(user)

	ch.RunRequestsSync([]isc.Request{skipReq, validReq}, "")

	// block has been created with only 1 request, calling 	`GetRequestReceiptsForBlock` must yield 1 receipt as expected
	bi := ch.GetLatestBlockInfo()
	require.EqualValues(t, 1, bi.TotalRequests)
	receipts := ch.GetRequestReceiptsForBlock(bi.BlockIndex())
	require.Len(t, receipts, 1)
}
