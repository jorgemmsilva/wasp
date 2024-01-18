package tests

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/hexutil"
	"github.com/iotaledger/iota.go/v4/tpkg"
	"github.com/iotaledger/wasp/clients/apiclient"
	"github.com/iotaledger/wasp/clients/chainclient"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

func parseBaseToken(s string) iotago.BaseToken {
	return iotago.BaseToken(lo.Must(strconv.ParseUint(s, 10, 64)))
}

func parseGasUnits(s string) gas.GasUnits {
	return gas.GasUnits(lo.Must(strconv.ParseUint(s, 10, 64)))
}

func testEstimateGasOnLedger(t *testing.T, env *ChainEnv) {
	// estimate on-ledger request, then send the same request, assert the gas used/fees match
	output := transaction.BasicOutputFromPostData(
		tpkg.RandEd25519Address(),
		isc.EmptyContractIdentity(),
		isc.RequestParameters{
			TargetAddress: env.Chain.ChainAddress(),
			Assets:        isc.NewAssetsBaseTokens(1 * isc.Million),
			Metadata: &isc.SendMetadata{
				Message:   accounts.FuncTransferAllowanceTo.Message(isc.NewAgentID(&iotago.Ed25519Address{})),
				Allowance: isc.NewAssetsBaseTokens(5000),
				GasBudget: 1e6,
			},
		},
		testutil.L1API,
	)

	estimatedReceipt, _, err := env.Chain.Cluster.WaspClient(0).ChainsApi.EstimateGasOnledger(context.Background(),
		env.Chain.ChainID.Bech32(env.Clu.L1Client().Bech32HRP()),
	).Request(apiclient.EstimateGasRequestOnledger{
		OutputBytes: hexutil.EncodeHex(lo.Must(testutil.L1API.Encode(output))),
	}).Execute()
	require.NoError(t, err)
	require.Empty(t, estimatedReceipt.ErrorMessage)

	keyPair, _, err := env.Clu.NewKeyPairWithFunds()
	require.NoError(t, err)

	feeCharged := parseBaseToken(estimatedReceipt.GasFeeCharged)

	client := env.Chain.Client(keyPair)
	par := chainclient.PostRequestParams{
		Transfer:  isc.NewAssetsBaseTokens(feeCharged),
		Allowance: isc.NewAssetsBaseTokens(5000),
	}
	gasBudget := parseGasUnits(estimatedReceipt.GasBurned)
	par.WithGasBudget(gasBudget)

	tx, err := client.PostRequest(
		accounts.FuncTransferAllowanceTo.Message(isc.NewAgentID(&iotago.Ed25519Address{})),
		par,
	)
	require.NoError(t, err)
	recs, err := env.Clu.MultiClient().WaitUntilAllRequestsProcessedSuccessfully(env.Chain.ChainID, tx, false, 10*time.Second)
	require.NoError(t, err)
	require.Equal(t, recs[0].GasBurned, estimatedReceipt.GasBurned)
	require.Equal(t, recs[0].GasFeeCharged, estimatedReceipt.GasFeeCharged)
}

func testEstimateGasOnLedgerNFT(t *testing.T, env *ChainEnv) {
	// estimate on-ledger request, using and NFT with minSD
	keyPair, addr, err := env.Clu.NewKeyPairWithFunds()
	require.NoError(t, err)

	metadata, err := hexutil.DecodeHex("0x7b227374616e64617264223a224952433237222c2276657273696f6e223a2276312e30222c226e616d65223a2254657374416761696e4e667432222c2274797065223a22696d6167652f6a706567222c22757269223a2268747470733a2f2f696d616765732e756e73706c6173682e636f6d2f70686f746f2d313639353539373737383238392d6663316635633731353935383f69786c69623d72622d342e302e3326697869643d4d3377784d6a4133664442384d48787761473930627931775957646c664878386647567566444238664878386641253344253344266175746f3d666f726d6174266669743d63726f7026773d3335343226713d3830227d")
	require.NoError(t, err)

	nftID, _, err := env.Clu.MintL1NFT(metadata, addr, keyPair)
	require.NoError(t, err)
	nft := &isc.NFT{
		ID:       iotago.NFTIDFromOutputID(nftID),
		Issuer:   addr,
		Metadata: iotago.MetadataFeatureEntries{"": metadata},
	}

	targetAgentID := isc.NewEthereumAddressAgentID(env.Chain.ChainID, common.Address{})

	output := transaction.NFTOutputFromPostData(
		tpkg.RandEd25519Address(),
		isc.EmptyContractIdentity(),
		isc.RequestParameters{
			Assets:                        isc.NewEmptyAssets(),
			AdjustToMinimumStorageDeposit: true,
			TargetAddress:                 env.Chain.ChainAddress(),
			Metadata: &isc.SendMetadata{
				Message:   accounts.FuncTransferAllowanceTo.Message(targetAgentID),
				Allowance: isc.NewEmptyAssets().AddNFTs(nft.ID),
				GasBudget: 1e6,
			},
			UnlockConditions: []iotago.UnlockCondition{
				&iotago.ExpirationUnlockCondition{
					ReturnAddress: addr,
					Slot:          testutil.L1API.TimeProvider().SlotFromTime(time.Now().Add(100 * time.Hour)),
				},
			},
		},
		nft,
		testutil.L1API,
	)

	estimatedReceipt, _, err := env.Chain.Cluster.WaspClient(0).ChainsApi.EstimateGasOnledger(context.Background(),
		env.Chain.ChainID.Bech32(env.Clu.L1Client().Bech32HRP()),
	).Request(apiclient.EstimateGasRequestOnledger{
		OutputBytes: hexutil.EncodeHex(lo.Must(testutil.L1API.Encode(output))),
	}).Execute()
	require.NoError(t, err)
	require.Empty(t, estimatedReceipt.ErrorMessage)

	client := env.Chain.Client(keyPair)
	par := chainclient.PostRequestParams{
		Transfer:                 isc.NewAssetsBaseTokens(output.BaseTokenAmount()),
		Allowance:                isc.NewEmptyAssets().AddNFTs(nft.ID),
		NFT:                      nft,
		AutoAdjustStorageDeposit: false,
	}
	gasBudget := parseGasUnits(estimatedReceipt.GasBurned)
	par.WithGasBudget(gasBudget)

	tx, err := client.PostRequest(accounts.FuncTransferAllowanceTo.Message(targetAgentID), par)
	require.NoError(t, err)
	recs, err := env.Clu.MultiClient().WaitUntilAllRequestsProcessedSuccessfully(env.Chain.ChainID, tx, false, 10*time.Second)
	require.NoError(t, err)
	require.Equal(t, recs[0].GasBurned, estimatedReceipt.GasBurned)
	require.Equal(t, recs[0].GasFeeCharged, estimatedReceipt.GasFeeCharged)
	require.Len(t, env.getAccountNFTs(targetAgentID), 1)
}

func testEstimateGasOffLedger(t *testing.T, env *ChainEnv) {
	// estimate off-ledger request, then send the same request, assert the gas used/fees match
	keyPair, _, err := env.Clu.NewKeyPairWithFunds()
	require.NoError(t, err)
	env.DepositFunds(10*isc.Million, keyPair)

	estimationReq := isc.NewOffLedgerRequest(
		env.Chain.ChainID,
		accounts.FuncTransferAllowanceTo.Message(isc.NewAgentID(&iotago.Ed25519Address{})),
		0,
		1e6,
	).WithAllowance(isc.NewAssetsBaseTokens(5000)).
		WithSender(keyPair.GetPublicKey())

	estimatedReceipt, _, err := env.Chain.Cluster.WaspClient(0).ChainsApi.EstimateGasOffledger(context.Background(),
		env.Chain.ChainID.Bech32(env.Clu.L1Client().Bech32HRP()),
	).Request(apiclient.EstimateGasRequestOffledger{
		RequestBytes: hexutil.EncodeHex(estimationReq.Bytes()),
	}).Execute()
	require.NoError(t, err)
	require.Empty(t, estimatedReceipt.ErrorMessage)

	client := env.Chain.Client(keyPair)
	par := chainclient.PostRequestParams{
		Allowance: isc.NewAssetsBaseTokens(5000),
	}
	par.WithGasBudget(1e6)

	req, err := client.PostOffLedgerRequest(
		context.Background(),
		accounts.FuncTransferAllowanceTo.Message(isc.NewAgentID(&iotago.Ed25519Address{})),
		par,
	)
	require.NoError(t, err)
	rec, err := env.Clu.MultiClient().WaitUntilRequestProcessedSuccessfully(env.Chain.ChainID, req.ID(), false, 10*time.Second)
	require.NoError(t, err)
	require.Equal(t, rec.GasBurned, estimatedReceipt.GasBurned)
	require.Equal(t, rec.GasFeeCharged, estimatedReceipt.GasFeeCharged)
}
