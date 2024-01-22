package origin_test

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/hexutil"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/origin"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/testutil/testmisc"
	"github.com/iotaledger/wasp/packages/testutil/utxodb"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm/core/migrations/allmigrations"
)

func TestOrigin(t *testing.T) {
	l1commitment := origin.L1Commitment(0, nil, 0, testutil.TokenInfo)
	store := state.NewStoreWithUniqueWriteMutex(mapdb.NewMapDB())
	initBlock := origin.InitChain(0, store, nil, 0, testutil.TokenInfo)
	latestBlock, err := store.LatestBlock()
	require.NoError(t, err)
	require.True(t, l1commitment.Equals(initBlock.L1Commitment()))
	require.True(t, l1commitment.Equals(latestBlock.L1Commitment()))
}

func TestCreateOrigin(t *testing.T) {
	var u *utxodb.UtxoDB
	var originTx *iotago.SignedTransaction
	var userKey *cryptolib.KeyPair
	var userAddr, stateAddr *iotago.Ed25519Address
	var err error
	var chainID isc.ChainID
	var originTxID iotago.TransactionID

	initTest := func() {
		u = utxodb.New(testutil.L1API)
		userKey = cryptolib.NewKeyPair()
		userAddr = userKey.GetPublicKey().AsEd25519Address()
		_, err2 := u.GetFundsFromFaucet(userAddr)
		require.NoError(t, err2)

		stateKey := cryptolib.NewKeyPair()
		stateAddr = stateKey.GetPublicKey().AsEd25519Address()

		require.EqualValues(t, utxodb.FundsFromFaucetAmount, u.GetAddressBalanceBaseTokens(userAddr))
		require.EqualValues(t, 0, u.GetAddressBalanceBaseTokens(stateAddr))
	}
	createOrigin := func() {
		allOutputs := u.GetUnspentOutputs(userAddr)

		originTx, _, chainID, err = origin.NewChainOriginTransaction(
			userKey,
			stateAddr,
			stateAddr,
			1000,
			1000,
			nil,
			allOutputs,
			u.SlotIndex(),
			allmigrations.DefaultScheme.LatestSchemaVersion(),
			testutil.L1APIProvider,
			testutil.TokenInfo,
		)
		require.NoError(t, err)

		err = u.AddToLedger(originTx)
		require.NoError(t, err)

		originTxID, err = originTx.Transaction.ID()
		require.NoError(t, err)

		txBack, ok := u.GetTransaction(originTxID)
		require.True(t, ok)
		txidBack, err2 := txBack.Transaction.ID()
		require.NoError(t, err2)
		require.EqualValues(t, originTxID, txidBack)

		t.Logf("New chain ID: %s", chainID.Bech32(testutil.L1API.ProtocolParameters().Bech32HRP()))
	}

	t.Run("create origin", func(t *testing.T) {
		initTest()
		createOrigin()

		anchor, _, err := transaction.GetAnchorFromTransaction(originTx.Transaction)
		require.NoError(t, err)
		require.True(t, anchor.IsOrigin)
		require.EqualValues(t, chainID, anchor.ChainID)
		require.EqualValues(t, 0, anchor.StateIndex)
		require.True(t, stateAddr.Equal(anchor.StateController))
		require.True(t, stateAddr.Equal(anchor.GovernanceController))

		// only one output is expected in the ledger under the address of chainID
		outs := u.GetUnspentOutputs(chainID.AsAddress())
		require.EqualValues(t, 1, len(outs))

		out := u.GetOutput(anchor.OutputID)
		require.NotNil(t, out)
	})
	t.Run("create init chain originTx", func(t *testing.T) {
		initTest()
		createOrigin()

		chainBaseTokens := originTx.Transaction.Outputs[0].BaseTokenAmount()

		t.Logf("chainBaseTokens: %d", chainBaseTokens)

		require.EqualValues(t, utxodb.FundsFromFaucetAmount-chainBaseTokens, int(u.GetAddressBalanceBaseTokens(userAddr)))
		require.EqualValues(t, 0, u.GetAddressBalanceBaseTokens(stateAddr))
		allOutputs := u.GetUnspentOutputs(chainID.AsAddress())
		require.EqualValues(t, 1, len(allOutputs))
	})
}

// Was used to find proper deposit values for a specific metadata according to the existing hashes.
func TestMetadataBad(t *testing.T) {
	t.SkipNow()
	metadataHex := "0300000001006102000000e60701006204000000ffffffff01006322000000010024ed2ed9d3682c9c4b801dd15103f73d1fe877224cb51c8b3def6f91b67f5067"
	metadataBin, err := hex.DecodeString(metadataHex)
	require.NoError(t, err)
	var initParams dict.Dict
	initParams, err = dict.FromBytes(metadataBin)
	require.NoError(t, err)
	require.NotNil(t, initParams)
	t.Logf("Dict=%v", initParams)
	initParams.Iterate(kv.EmptyPrefix, func(key kv.Key, value []byte) bool {
		t.Logf("Dict, %v ===> %v", key, value)
		return true
	})

	for deposit := iotago.BaseToken(0); deposit <= 10*isc.Million; deposit++ {
		db := mapdb.NewMapDB()
		st := state.NewStoreWithUniqueWriteMutex(db)
		block1A := origin.InitChain(0, st, initParams, deposit, testutil.TokenInfo)
		block1B := origin.InitChain(0, st, initParams, 10*isc.Million-deposit, testutil.TokenInfo)
		block1C := origin.InitChain(0, st, initParams, 10*isc.Million+deposit, testutil.TokenInfo)
		block2A := origin.InitChain(0, st, nil, deposit, testutil.TokenInfo)
		block2B := origin.InitChain(0, st, nil, 10*isc.Million-deposit, testutil.TokenInfo)
		block2C := origin.InitChain(0, st, nil, 10*isc.Million+deposit, testutil.TokenInfo)
		t.Logf("Block0, deposit=%v => %v %v %v / %v %v %v", deposit,
			block1A.L1Commitment(), block1B.L1Commitment(), block1C.L1Commitment(),
			block2A.L1Commitment(), block2B.L1Commitment(), block2C.L1Commitment(),
		)
	}
}

func TestDictBytes(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		key := rapid.SliceOfBytesMatching(".*").Draw(t, "key")
		val := rapid.SliceOfBytesMatching(".+").Draw(t, "val")
		d := dict.New()
		d.Set(kv.Key(key), val)
		b := d.Bytes()
		d2, err := dict.FromBytes(b)
		require.NoError(t, err)
		require.Equal(t, d, d2)
	})
}

// example values taken from a test on the testnet
func TestMismatchOriginCommitment(t *testing.T) {
	store := state.NewStoreWithUniqueWriteMutex(mapdb.NewMapDB())
	oid, err := iotago.OutputIDFromHexString("0xcf72dd6a8c8cd76eab93c80ae192677a17c554b91334a41bed5079eff37effc4000000000000")
	require.NoError(t, err)
	originMetadata, err := hexutil.DecodeHex("0x03016102e607016204ffffffff016322010024ed2ed9d3682c9c4b801dd15103f73d1fe877224cb51c8b3def6f91b67f5067")
	require.NoError(t, err)
	aoStateMetadata, err := hexutil.DecodeHex("0x01000000006e55672af085d73ea0ed646f280a26e0eba053df10f439378fe4e99e0fb8774600761da7c0402da8640000000100000000010000000100000000")
	require.NoError(t, err)
	_, sender, err := iotago.ParseBech32("rms1qqjw6tke6d5ze8ztsqwaz5gr7u73l6rhyfxt28yt8hhklydk0agxwgerk65")
	require.NoError(t, err)
	_, stateController, err := iotago.ParseBech32("rms1qrkrlggl2plwfvxyuuyj55gw48ws0xwtteydez8y8e03elm3xf38gf7eq5r")
	require.NoError(t, err)
	_, govController, err := iotago.ParseBech32("rms1qqjw6tke6d5ze8ztsqwaz5gr7u73l6rhyfxt28yt8hhklydk0agxwgerk65")
	require.NoError(t, err)

	ao := isc.NewChainOutputs(
		&iotago.AnchorOutput{
			Amount:     10000000,
			Mana:       0,
			AnchorID:   iotago.EmptyAnchorID,
			StateIndex: 0,
			UnlockConditions: iotago.AnchorOutputUnlockConditions{
				&iotago.StateControllerAddressUnlockCondition{Address: stateController},
				&iotago.GovernorAddressUnlockCondition{Address: govController},
			},
			Features: []iotago.Feature{
				&iotago.SenderFeature{
					Address: sender,
				},
				&iotago.StateMetadataFeature{
					Entries: map[iotago.StateMetadataFeatureEntriesKey]iotago.StateMetadataFeatureEntriesValue{
						"": aoStateMetadata,
					},
				},
				&iotago.MetadataFeature{
					Entries: map[iotago.MetadataFeatureEntriesKey]iotago.MetadataFeatureEntriesValue{
						"": originMetadata,
					},
				},
			},
		},
		oid,
		nil,
		iotago.OutputID{},
	)

	_, err = origin.InitChainByAnchorOutput(store, ao, testutil.L1APIProvider, testutil.TokenInfo)
	testmisc.RequireErrorToBe(t, err, "l1Commitment mismatch between originAO / originBlock")
}
