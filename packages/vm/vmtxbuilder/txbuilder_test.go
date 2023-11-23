package vmtxbuilder

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/tpkg"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/testutil/testiotago"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/util/panicutil"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/vmexceptions"
)

var dummyStateMetadata = []byte("foobar")

type mockAccountContractRead struct {
	assets             *isc.FungibleTokens
	nativeTokenOutputs map[iotago.NativeTokenID]*iotago.BasicOutput
	changeInSD         int64
}

func (m *mockAccountContractRead) Read() AccountsContractRead {
	return AccountsContractRead{
		NativeTokenOutput: func(id iotago.FoundryID) (*iotago.BasicOutput, iotago.OutputID) {
			return m.nativeTokenOutputs[id], iotago.OutputID{}
		},
		FoundryOutput: func(uint32) (*iotago.FoundryOutput, iotago.OutputID) {
			return nil, iotago.OutputID{}
		},
		NFTOutput: func(id iotago.NFTID) (*iotago.NFTOutput, iotago.OutputID) {
			return nil, iotago.OutputID{}
		},
		TotalFungibleTokens: func() *isc.FungibleTokens {
			return m.adjustSD(m.assets)
		},
	}
}

// mimics the behavior of accounts.AdjustAccountBaseTokens
func (m *mockAccountContractRead) adjustSD(assets *isc.FungibleTokens) *isc.FungibleTokens {
	if m.changeInSD > 0 {
		assets = assets.Clone()
		assets.BaseTokens += iotago.BaseToken(m.changeInSD)
	}
	if m.changeInSD < 0 {
		assets = assets.Clone()
		assets.BaseTokens -= iotago.BaseToken(-m.changeInSD)
	}
	return assets
}

func newMockAccountsContractRead(anchor *iotago.AnchorOutput) *mockAccountContractRead {
	sd := lo.Must(testutil.L1API.StorageScoreStructure().MinDeposit(anchor))
	assets := isc.NewFungibleTokens(anchor.BaseTokenAmount()-sd, nil)
	return &mockAccountContractRead{
		assets:             assets,
		nativeTokenOutputs: make(map[iotago.FoundryID]*iotago.BasicOutput),
	}
}

func buildTxEssence(txb *AnchorTransactionBuilder, mockedAccounts *mockAccountContractRead) *iotago.Transaction {
	_, _, changeInSD := txb.ChangeInSD(dummyStateMetadata, 0)
	mockedAccounts.changeInSD = changeInSD
	essence, _ := txb.BuildTransactionEssence(dummyStateMetadata, 0)
	txb.MustBalanced()
	return essence
}

func TestTxBuilderBasic(t *testing.T) {
	const initialTotalBaseTokens = 10 * isc.Million
	addr := tpkg.RandEd25519Address()
	anchorID := testiotago.RandAnchorID()
	anchor := &iotago.AnchorOutput{
		Amount:   initialTotalBaseTokens,
		AnchorID: anchorID,
		UnlockConditions: iotago.AnchorOutputUnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{Address: addr},
			&iotago.GovernorAddressUnlockCondition{Address: addr},
		},
		StateIndex: 0,
		Features: iotago.AnchorOutputFeatures{
			&iotago.SenderFeature{
				Address: anchorID.ToAddress(),
			},
			&iotago.StateMetadataFeature{
				Entries: iotago.StateMetadataFeatureEntries{"": dummyStateMetadata},
			},
		},
	}
	anchorOutputID := tpkg.RandOutputID(0)

	accountID := testiotago.RandAccountID()
	account := &iotago.AccountOutput{
		AccountID:      accountID,
		FoundryCounter: 0,
		UnlockConditions: iotago.AccountOutputUnlockConditions{
			&iotago.AddressUnlockCondition{Address: anchorID.ToAddress()},
		},
		Features: iotago.AccountOutputFeatures{
			&iotago.SenderFeature{Address: anchorID.ToAddress()},
		},
	}
	var err error
	account.Amount, err = testutil.L1API.StorageScoreStructure().MinDeposit(account)
	require.NoError(t, err)
	anchor.Amount -= account.Amount
	accountOutputID := tpkg.RandOutputID(1)

	t.Run("deposits", func(t *testing.T) {
		mockedAccounts := newMockAccountsContractRead(anchor)
		txb := NewAnchorTransactionBuilder(
			isc.NewChainOutputs(
				anchor,
				anchorOutputID,
				account,
				accountOutputID,
			),
			mockedAccounts.Read(),
		)
		essence := buildTxEssence(txb, mockedAccounts)
		require.EqualValues(t, 2, txb.numInputs())
		require.EqualValues(t, 2, txb.numOutputs())
		require.False(t, txb.InputsAreFull())
		require.False(t, txb.outputsAreFull())

		require.EqualValues(t, 2, len(essence.TransactionEssence.Inputs))
		require.EqualValues(t, 2, len(essence.Outputs))

		_, err := testutil.L1API.Encode(essence)
		require.NoError(t, err)

		// consume a request that sends 1Mi funds
		req1, err := isc.OnLedgerFromUTXO(&iotago.BasicOutput{
			Amount: 1 * isc.Million,
		}, iotago.OutputID{})
		require.NoError(t, err)
		txb.Consume(req1)
		mockedAccounts.assets.AddBaseTokens(req1.Output().BaseTokenAmount())

		essence = buildTxEssence(txb, mockedAccounts)
		require.Len(t, essence.Outputs, 2)
		require.EqualValues(t, essence.Outputs[0].BaseTokenAmount(), anchor.BaseTokenAmount()+req1.Output().BaseTokenAmount())

		// consume a request that sends 1Mi, 1 NFT, and 4 native tokens
		nftID := tpkg.RandNFTAddress().NFTID()
		nativeTokenID1 := testiotago.RandNativeTokenID()

		req2, err := isc.OnLedgerFromUTXO(&iotago.NFTOutput{
			Amount: 1 * isc.Million,
			NFTID:  nftID,
			Features: iotago.NFTOutputFeatures{
				&iotago.NativeTokenFeature{ID: nativeTokenID1, Amount: big.NewInt(1)},
			},
		}, iotago.OutputID{})
		require.NoError(t, err)
		totalSDBaseTokensUsedToSplitAssets := txb.Consume(req2)

		// deduct SD costs of creating the internal accounting outputs
		mockedAccounts.assets.Add(&req2.Assets().FungibleTokens)
		mockedAccounts.assets.Spend(isc.NewFungibleTokens(totalSDBaseTokensUsedToSplitAssets, nil))

		essence = buildTxEssence(txb, mockedAccounts)
		require.Len(t, essence.Outputs, 4) // 1 anchor AO, 1 account AO, 1 NFT internal Output, 1 NativeTokens internal outputs
		require.EqualValues(t, essence.Outputs[0].BaseTokenAmount(), anchor.BaseTokenAmount()+req1.Output().BaseTokenAmount()+req2.Output().BaseTokenAmount()-totalSDBaseTokensUsedToSplitAssets)
	})
}

func TestTxBuilderConsistency(t *testing.T) {
	const initialTotalBaseTokens = 10000 * isc.Million
	addr := tpkg.RandEd25519Address()
	anchorID := testiotago.RandAnchorID()
	anchor := &iotago.AnchorOutput{
		Amount:   initialTotalBaseTokens,
		AnchorID: anchorID,
		UnlockConditions: iotago.AnchorOutputUnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{Address: addr},
			&iotago.GovernorAddressUnlockCondition{Address: addr},
		},
		StateIndex: 0,
		Features: iotago.AnchorOutputFeatures{
			&iotago.SenderFeature{
				Address: anchorID.ToAddress(),
			},
			&iotago.StateMetadataFeature{
				Entries: iotago.StateMetadataFeatureEntries{"": dummyStateMetadata},
			},
		},
	}
	anchorOutputID := tpkg.RandOutputID(0)

	accountID := testiotago.RandAccountID()
	account := &iotago.AccountOutput{
		AccountID:      accountID,
		FoundryCounter: 0,
		UnlockConditions: iotago.AccountOutputUnlockConditions{
			&iotago.AddressUnlockCondition{Address: anchorID.ToAddress()},
		},
		Features: iotago.AccountOutputFeatures{
			&iotago.SenderFeature{Address: anchorID.ToAddress()},
		},
	}
	account.Amount = lo.Must(testutil.L1API.StorageScoreStructure().MinDeposit(account))
	anchor.Amount -= account.Amount
	accountOutputID := tpkg.RandOutputID(1)

	initTest := func(numTokenIDs int) (*AnchorTransactionBuilder, *mockAccountContractRead, []iotago.NativeTokenID) {
		mockedAccounts := newMockAccountsContractRead(anchor)
		txb := NewAnchorTransactionBuilder(
			isc.NewChainOutputs(
				anchor,
				anchorOutputID,
				account,
				accountOutputID,
			),
			mockedAccounts.Read(),
		)

		nativeTokenIDs := make([]iotago.NativeTokenID, 0)
		for i := 0; i < numTokenIDs; i++ {
			nativeTokenIDs = append(nativeTokenIDs, testiotago.RandNativeTokenID())
		}
		return txb, mockedAccounts, nativeTokenIDs
	}

	// return deposit in BaseToken
	consumeUTXO := func(t *testing.T, txb *AnchorTransactionBuilder, id iotago.NativeTokenID, amountNative uint64, mockedAccounts *mockAccountContractRead) {
		out := transaction.MakeBasicOutput(
			txb.inputs.AnchorOutput.AnchorID.ToAddress(),
			nil,
			isc.NewFungibleTokens(0, iotago.NativeTokenSum{id: big.NewInt(int64(amountNative))}),
			0,
			nil,
			nil,
		)
		req, err := isc.OnLedgerFromUTXO(transaction.AdjustToMinimumStorageDeposit(out, testutil.L1API), iotago.OutputID{})
		require.NoError(t, err)
		sdCost := txb.Consume(req)
		mockedAccounts.assets.Add(&req.Assets().FungibleTokens)
		mockedAccounts.assets.Spend(isc.NewFungibleTokens(sdCost, nil))
		buildTxEssence(txb, mockedAccounts)
	}

	addOutput := func(txb *AnchorTransactionBuilder, amount uint64, nativeTokenID iotago.NativeTokenID, mockedAccounts *mockAccountContractRead) {
		outAssets := isc.NewFungibleTokens(
			1*isc.Million,
			iotago.NativeTokenSum{nativeTokenID: new(big.Int).SetUint64(amount)},
		)
		out := transaction.BasicOutputFromPostData(
			txb.inputs.AnchorOutput.AnchorID.ToAddress(),
			isc.ContractIdentityFromHname(isc.Hn("test")),
			isc.RequestParameters{
				TargetAddress:                 tpkg.RandEd25519Address(),
				Assets:                        outAssets.ToAssets(),
				AdjustToMinimumStorageDeposit: true,
				Metadata:                      &isc.SendMetadata{},
			},
			testutil.L1API,
		)
		sdAdjust := txb.AddOutput(out)
		if !mockedAccounts.assets.Spend(outAssets) {
			panic("out of balance in chain output")
		}
		if sdAdjust < 0 {
			mockedAccounts.assets.Spend(isc.NewFungibleTokens(iotago.BaseToken(-sdAdjust), nil))
		} else {
			mockedAccounts.assets.AddBaseTokens(iotago.BaseToken(sdAdjust))
		}
		buildTxEssence(txb, mockedAccounts)
	}

	t.Run("consistency check", func(t *testing.T) {
		const runTimes = 100
		const testAmount = 10
		const numTokenIDs = 4

		txb, mockedAccounts, nativeTokenIDs := initTest(numTokenIDs)
		for i := 0; i < runTimes; i++ {
			idx := rand.Intn(numTokenIDs)
			consumeUTXO(t, txb, nativeTokenIDs[idx], testAmount, mockedAccounts)
		}

		essence := buildTxEssence(txb, mockedAccounts)

		essenceBytes, err := testutil.L1API.Encode(essence)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})

	runConsume := func(txb *AnchorTransactionBuilder, nativeTokenIDs []iotago.NativeTokenID, numRun int, amountNative uint64, mockedAccounts *mockAccountContractRead) {
		for i := 0; i < numRun; i++ {
			idx := i % len(nativeTokenIDs)
			consumeUTXO(t, txb, nativeTokenIDs[idx], amountNative, mockedAccounts)
			buildTxEssence(txb, mockedAccounts)
		}
	}

	t.Run("exceed inputs", func(t *testing.T) {
		const runTimes = 150
		const testAmount = 10
		const numTokenIDs = 4

		txb, mockedAccounts, nativeTokenIDs := initTest(numTokenIDs)
		err := panicutil.CatchPanicReturnError(func() {
			runConsume(txb, nativeTokenIDs, runTimes, testAmount, mockedAccounts)
		}, vmexceptions.ErrInputLimitExceeded)
		require.Error(t, err, vmexceptions.ErrInputLimitExceeded)
	})
	t.Run("exceeded outputs", func(t *testing.T) {
		const runTimesInputs = 120
		const runTimesOutputs = 130
		const numTokenIDs = 5

		txb, mockedAccounts, nativeTokenIDs := initTest(numTokenIDs)
		runConsume(txb, nativeTokenIDs, runTimesInputs, 10, mockedAccounts)
		buildTxEssence(txb, mockedAccounts)

		err := panicutil.CatchPanicReturnError(func() {
			for i := 0; i < runTimesOutputs; i++ {
				idx := rand.Intn(numTokenIDs)
				addOutput(txb, 1, nativeTokenIDs[idx], mockedAccounts)
			}
		}, vmexceptions.ErrOutputLimitExceeded)
		require.Error(t, err, vmexceptions.ErrOutputLimitExceeded)
	})
	t.Run("randomize", func(t *testing.T) {
		const runTimes = 30
		const numTokenIDs = 5

		txb, mockedAccounts, nativeTokenIDs := initTest(numTokenIDs)
		for _, id := range nativeTokenIDs {
			consumeUTXO(t, txb, id, 10, mockedAccounts)
		}

		for i := 0; i < runTimes; i++ {
			idx1 := rand.Intn(numTokenIDs)
			consumeUTXO(t, txb, nativeTokenIDs[idx1], 1, mockedAccounts)
			idx2 := rand.Intn(numTokenIDs)
			if mockedAccounts.assets.NativeTokens.ValueOrBigInt0(nativeTokenIDs[idx2]).Uint64() > 0 {
				addOutput(txb, 1, nativeTokenIDs[idx2], mockedAccounts)
			}
		}
		essence := buildTxEssence(txb, mockedAccounts)

		essenceBytes, err := testutil.L1API.Encode(essence)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
	t.Run("send some of the tokens in balance", func(t *testing.T) {
		txb, mockedAccounts, nativeTokenIDs := initTest(5)
		setNativeTokenAccountsBalance := func(id iotago.NativeTokenID, amount *big.Int) {
			mockedAccounts.assets.AddNativeTokens(id, amount)
			// create internal accounting outputs with 0 base tokens (they must be updated in the output side)
			out := txb.newInternalTokenOutput(anchorID, id)
			out.FeatureSet().NativeToken().Amount = new(big.Int).Set(amount)
			mockedAccounts.nativeTokenOutputs[id] = out
		}

		// send 90 < 100 which is on-chain. 10 must be left and storage deposit should not disappear
		for i := range nativeTokenIDs {
			setNativeTokenAccountsBalance(nativeTokenIDs[i], big.NewInt(100))
			addOutput(txb, 90, nativeTokenIDs[i], mockedAccounts)
		}
		essence := buildTxEssence(txb, mockedAccounts)

		require.EqualValues(t, 7, len(essence.TransactionEssence.Inputs))
		require.EqualValues(t, 12, len(essence.Outputs)) // 6 + 5 internal outputs with the 10 remaining tokens

		essenceBytes, err := testutil.L1API.Encode(essence)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})

	t.Run("test consistency - consume send out, consume again", func(t *testing.T) {
		txb, mockedAccounts, nativeTokenIDs := initTest(1)
		tokenID := nativeTokenIDs[0]
		consumeUTXO(t, txb, tokenID, 1, mockedAccounts)
		addOutput(txb, 1, tokenID, mockedAccounts)
		consumeUTXO(t, txb, tokenID, 1, mockedAccounts)

		essence := buildTxEssence(txb, mockedAccounts)

		essenceBytes, err := testutil.L1API.Encode(essence)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
}

func TestFoundries(t *testing.T) {
	const initialTotalBaseTokens = 10*isc.Million + governance.DefaultMinBaseTokensOnCommonAccount
	addr := tpkg.RandEd25519Address()
	anchorID := testiotago.RandAnchorID()
	anchor := &iotago.AnchorOutput{
		Amount:   initialTotalBaseTokens,
		AnchorID: anchorID,
		UnlockConditions: iotago.AnchorOutputUnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{Address: addr},
			&iotago.GovernorAddressUnlockCondition{Address: addr},
		},
		StateIndex: 0,
		Features: iotago.AnchorOutputFeatures{
			&iotago.SenderFeature{
				Address: anchorID.ToAddress(),
			},
			&iotago.StateMetadataFeature{
				Entries: iotago.StateMetadataFeatureEntries{"": dummyStateMetadata},
			},
		},
	}
	anchorOutputID := tpkg.RandOutputID(0)

	accountID := testiotago.RandAccountID()
	account := &iotago.AccountOutput{
		AccountID:      accountID,
		FoundryCounter: 0,
		UnlockConditions: iotago.AccountOutputUnlockConditions{
			&iotago.AddressUnlockCondition{Address: anchorID.ToAddress()},
		},
		Features: iotago.AccountOutputFeatures{
			&iotago.SenderFeature{Address: anchorID.ToAddress()},
		},
	}
	account.Amount = lo.Must(testutil.L1API.StorageScoreStructure().MinDeposit(account))
	anchor.Amount -= account.Amount
	accountOutputID := tpkg.RandOutputID(1)

	var nativeTokenIDs []iotago.NativeTokenID
	var txb *AnchorTransactionBuilder
	var numTokenIDs int

	var mockedAccounts *mockAccountContractRead
	initTest := func() {
		mockedAccounts = newMockAccountsContractRead(anchor)
		txb = NewAnchorTransactionBuilder(
			isc.NewChainOutputs(
				anchor,
				anchorOutputID,
				account,
				accountOutputID,
			),
			mockedAccounts.Read(),
		)

		nativeTokenIDs = make([]iotago.NativeTokenID, 0)

		for i := 0; i < numTokenIDs; i++ {
			nativeTokenIDs = append(nativeTokenIDs, testiotago.RandNativeTokenID())
		}
	}
	createNFoundries := func(n int) {
		for i := 0; i < n; i++ {
			sn, storageDeposit := txb.CreateNewFoundry(
				&iotago.SimpleTokenScheme{MaximumSupply: big.NewInt(10_000_000), MeltedTokens: util.Big0, MintedTokens: util.Big0},
				nil,
			)
			require.EqualValues(t, i+1, int(sn))

			mockedAccounts.assets.BaseTokens -= storageDeposit
			buildTxEssence(txb, mockedAccounts)
		}
	}
	t.Run("create foundry ok", func(t *testing.T) {
		initTest()
		createNFoundries(3)
		essence := buildTxEssence(txb, mockedAccounts)
		essenceBytes, err := testutil.L1API.Encode(essence)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
}

func TestSerDe(t *testing.T) {
	t.Run("serde BasicOutput", func(t *testing.T) {
		reqMetadata := isc.RequestMetadata{
			SenderContract: isc.EmptyContractIdentity(),
			TargetContract: 0,
			EntryPoint:     0,
			Params:         dict.New(),
			Allowance:      isc.NewEmptyAssets(),
			GasBudget:      0,
		}
		out := transaction.AdjustToMinimumStorageDeposit(
			transaction.MakeBasicOutput(
				&iotago.Ed25519Address{},
				&iotago.Ed25519Address{1, 2, 3},
				isc.NewEmptyFungibleTokens(),
				0,
				&reqMetadata,
				nil,
			),
			testutil.L1API,
		)
		data, err := testutil.L1API.Encode(out)
		require.NoError(t, err)
		outBack := &iotago.BasicOutput{}
		_, err = testutil.L1API.Decode(data, &outBack)
		require.NoError(t, err)
		condSet := out.UnlockConditions.MustSet()
		condSetBack := outBack.UnlockConditions.MustSet()
		require.True(t, condSet[iotago.UnlockConditionAddress].Equal(condSetBack[iotago.UnlockConditionAddress]))
		require.EqualValues(t, out.BaseTokenAmount(), outBack.Amount)
		require.Nil(t, outBack.FeatureSet().NativeToken())
		require.True(t, outBack.Features.Equal(out.Features))
	})
	t.Run("serde FoundryOutput", func(t *testing.T) {
		out := &iotago.FoundryOutput{
			UnlockConditions: iotago.FoundryOutputUnlockConditions{
				&iotago.ImmutableAccountUnlockCondition{Address: tpkg.RandAccountAddress()},
			},
			Amount:       1337,
			SerialNumber: 5,
			TokenScheme: &iotago.SimpleTokenScheme{
				MintedTokens:  big.NewInt(200),
				MeltedTokens:  big.NewInt(0),
				MaximumSupply: big.NewInt(2000),
			},
			Features: nil,
		}
		data, err := testutil.L1API.Encode(out)
		require.NoError(t, err)
		outBack := &iotago.FoundryOutput{}
		_, err = testutil.L1API.Decode(data, &outBack)
		require.NoError(t, err)
		require.True(t, identicalFoundries(out, outBack))
	})
}
