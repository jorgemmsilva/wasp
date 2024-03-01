package solo_test

import (
	"math/big"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
)

// This test is an example of how to generate a snapshot from a Solo chain.
// The snapshot is especially useful to test migrations.
func TestSaveSnapshot(t *testing.T) {
	// skipped by default because the generated dump is fairly large
	if os.Getenv("ENABLE_SOLO_SNAPSHOT") == "" {
		t.SkipNow()
	}

	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true, Debug: true})
	ch := env.NewChain()
	ch.MustDepositBaseTokensToL2(2*isc.Million, ch.OriginatorKeyPair)

	// create foundry and native tokens on L2
	sn, nativeTokenID, err := ch.NewFoundryParams(big.NewInt(1000)).CreateFoundry()
	require.NoError(t, err)
	// mint some tokens for the user
	err = ch.MintTokens(sn, big.NewInt(1000), ch.OriginatorKeyPair)
	require.NoError(t, err)

	_, err = ch.GetNativeTokenIDByFoundrySN(sn)
	require.NoError(t, err)
	ch.AssertL2NativeTokens(ch.OriginatorAgentID, nativeTokenID, 1000)

	// create NFT on L1 and deposit on L2
	nft, _, err := ch.Env.MintNFTL1(ch.OriginatorKeyPair, ch.OriginatorAddress, iotago.MetadataFeatureEntries{
		"": []byte("foobar"), // TODO does this need some special key?
	})
	require.NoError(t, err)
	_, err = ch.PostRequestSync(
		solo.NewCallParams(accounts.FuncDeposit.Message()).
			WithNFT(nft).
			AddBaseTokens(10*isc.Million).
			WithMaxAffordableGasBudget(),
		ch.OriginatorKeyPair)
	require.NoError(t, err)

	require.NotEmpty(t, ch.L2NFTs(ch.OriginatorAgentID))

	ch.Env.SaveSnapshot(ch.Env.TakeSnapshot(), "snapshot.db")
}

// This test is an example of how to restore a Solo snapshot.
// The snapshot is especially useful to test migrations.
func TestLoadSnapshot(t *testing.T) {
	// skipped because this is just an example, the dump is not committed
	t.SkipNow()

	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true, Debug: true})
	env.RestoreSnapshot(env.LoadSnapshot("snapshot.db"))

	ch := env.GetChainByName("chain1")

	require.EqualValues(t, 5, ch.LatestBlockIndex())

	nativeTokenID, err := ch.GetNativeTokenIDByFoundrySN(1)
	require.NoError(t, err)
	ch.AssertL2NativeTokens(ch.OriginatorAgentID, nativeTokenID, 1000)
}
