package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/client/chainclient"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

// executed in cluster_test.go
func testMaintenance(t *testing.T, e *chainEnv) {
	e.deployWasmInccounter(0)
	ownerWallet, ownerAddr, err := e.Clu.NewKeyPairWithFunds()
	require.NoError(t, err)
	ownerAgentID := isc.NewAgentID(ownerAddr)
	e.DepositFunds(10*isc.Million, ownerWallet)
	ownerSCClient := e.Chain.SCClient(governance.Contract.Hname(), ownerWallet)
	ownerIncCounterSCClient := e.Chain.SCClient(incHname, ownerWallet)

	userWallet, _, err := e.Clu.NewKeyPairWithFunds()
	require.NoError(t, err)
	e.DepositFunds(10*isc.Million, userWallet)
	userSCClient := e.Chain.SCClient(governance.Contract.Hname(), userWallet)
	userIncCounterSCClient := e.Chain.SCClient(incHname, userWallet)

	// set owner of the chain
	{
		originatorSCClient := e.Chain.SCClient(governance.Contract.Hname(), e.Chain.OriginatorKeyPair)
		tx, err := originatorSCClient.PostRequest(governance.FuncDelegateChainOwnership.Name, chainclient.PostRequestParams{
			Args: dict.Dict{
				governance.ParamChainOwner: codec.Encode(ownerAgentID),
			},
		})
		require.NoError(t, err)
		_, err = e.Clu.MultiClient().WaitUntilAllRequestsProcessedSuccessfully(e.Chain.ChainID, tx, 10*time.Second)
		require.NoError(t, err)

		req, err := ownerSCClient.PostOffLedgerRequest(governance.FuncClaimChainOwnership.Name)
		require.NoError(t, err)
		_, err = e.Clu.MultiClient().WaitUntilRequestProcessedSuccessfully(e.Chain.ChainID, req.ID(), 10*time.Second)
		require.NoError(t, err)
	}

	// call the gov "maintenance status view", check it is OFF
	{
		ret, err := ownerSCClient.CallView(governance.ViewGetMaintenanceStatus.Name, nil)
		require.NoError(t, err)
		maintenanceStatus := codec.MustDecodeBool(ret.MustGet(governance.VarMaintenanceStatus))
		require.False(t, maintenanceStatus)
	}

	// test non-chain owner cannot call init maintenance
	{
		req, err := userSCClient.PostOffLedgerRequest(governance.FuncStartMaintenance.Name)
		require.NoError(t, err)
		rec, err := e.Clu.MultiClient().WaitUntilRequestProcessed(e.Chain.ChainID, req.ID(), 10*time.Second)
		require.NoError(t, err)
		require.Error(t, rec.Error)
	}

	// owner can start maintenance mode
	{
		req, err := ownerSCClient.PostOffLedgerRequest(governance.FuncStartMaintenance.Name)
		require.NoError(t, err)
		_, err = e.Clu.MultiClient().WaitUntilRequestProcessedSuccessfully(e.Chain.ChainID, req.ID(), 10*time.Second)
		require.NoError(t, err)
	}

	// call the gov "maintenance status view", check it is ON
	{
		ret, err := ownerSCClient.CallView(governance.ViewGetMaintenanceStatus.Name, nil)
		require.NoError(t, err)
		maintenanceStatus := codec.MustDecodeBool(ret.MustGet(governance.VarMaintenanceStatus))
		require.True(t, maintenanceStatus)
	}

	// get the current block number
	blockIndex, err := e.Chain.BlockIndex()
	require.NoError(t, err)

	// calls to non-maintenance endpoints are not processed
	notProccessedReq1, err := userIncCounterSCClient.PostOffLedgerRequest(incrementFuncName)
	require.NoError(t, err)
	time.Sleep(10 * time.Second) // not ideal, but I don't think there is a good way to wait for something that will NOT be processed
	rec, err := e.Chain.GetRequestReceipt(notProccessedReq1.ID())
	require.Regexp(t, `.*"Code":404.*`, err.Error())
	require.Nil(t, rec)

	// calls to non-maintenance endpoints are not processed, even when done by the chain owner
	notProccessedReq2, err := ownerIncCounterSCClient.PostOffLedgerRequest(incrementFuncName)
	require.NoError(t, err)
	time.Sleep(10 * time.Second) // not ideal, but I don't think there is a good way to wait for something that will NOT be processed
	rec, err = e.Chain.GetRequestReceipt(notProccessedReq2.ID())
	require.Regexp(t, `.*"Code":404.*`, err.Error())
	require.Nil(t, rec)

	// assert that block number is still the same
	blockIndex2, err := e.Chain.BlockIndex()
	require.NoError(t, err)
	require.EqualValues(t, blockIndex, blockIndex2)

	// calls to governance are processed (try changing fees for example)
	newGasFeePolicy := gas.GasFeePolicy{
		GasFeeTokenID:     iotago.NativeTokenID{},
		GasPerToken:       10,
		ValidatorFeeShare: 1,
	}
	{
		req, err := ownerSCClient.PostOffLedgerRequest(governance.FuncSetFeePolicy.Name, chainclient.PostRequestParams{
			Args: dict.Dict{
				governance.ParamFeePolicyBytes: newGasFeePolicy.Bytes(),
			},
		})
		require.NoError(t, err)
		_, err = e.Clu.MultiClient().WaitUntilRequestProcessedSuccessfully(e.Chain.ChainID, req.ID(), 10*time.Second)
		require.NoError(t, err)
	}

	// calls to governance from non-owners should be processed, but fail
	{
		req, err := userSCClient.PostOffLedgerRequest(governance.FuncSetFeePolicy.Name, chainclient.PostRequestParams{
			Args: dict.Dict{
				governance.ParamFeePolicyBytes: newGasFeePolicy.Bytes(),
			},
		})
		require.NoError(t, err)
		receipt, err := e.Clu.MultiClient().WaitUntilRequestProcessed(e.Chain.ChainID, req.ID(), 10*time.Second)
		require.NoError(t, err)
		require.Error(t, receipt.Error)
	}

	// test non-chain owner cannot call stop maintenance
	{
		req, err := userSCClient.PostOffLedgerRequest(governance.FuncStopMaintenance.Name)
		require.NoError(t, err)
		rec, err := e.Clu.MultiClient().WaitUntilRequestProcessed(e.Chain.ChainID, req.ID(), 10*time.Second)
		require.NoError(t, err)
		require.Error(t, rec.Error)
	}

	// owner can stop maintenance mode
	{
		req, err := ownerSCClient.PostOffLedgerRequest(governance.FuncStopMaintenance.Name)
		require.NoError(t, err)
		_, err = e.Clu.MultiClient().WaitUntilRequestProcessedSuccessfully(e.Chain.ChainID, req.ID(), 10*time.Second)
		require.NoError(t, err)
	}

	// normal requests are now processed successfully (pending requests issued during maintenance should be processed now)
	{
		_, err = e.Clu.MultiClient().WaitUntilRequestProcessedSuccessfully(e.Chain.ChainID, notProccessedReq1.ID(), 10*time.Second)
		require.NoError(t, err)
		_, err = e.Clu.MultiClient().WaitUntilRequestProcessedSuccessfully(e.Chain.ChainID, notProccessedReq2.ID(), 10*time.Second)
		require.NoError(t, err)
		e.expectCounter(2)
	}
}
