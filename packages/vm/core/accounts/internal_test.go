package accounts_test

import (
	"math/big"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/tpkg"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/migrations/allmigrations"
)

func TestAccounts(t *testing.T) {
	// execute tests on all schema versions
	for v := isc.SchemaVersion(0); v <= allmigrations.DefaultScheme.LatestSchemaVersion(); v++ {
		testCreditDebit1(t, v)
		testCreditDebit2(t, v)
		testCreditDebit3(t, v)
		testCreditDebit4(t, v)
		testCreditDebit5(t, v)
		testCreditDebit6(t, v)
		testCreditDebit7(t, v)
		testMoveAll(t, v)
		testDebitAll(t, v)
		testTransferNFTs(t, v)
		testCreditDebitNFT1(t, v)
	}
}

func knownAgentID(b byte, h uint32) isc.AgentID {
	var chainID isc.ChainID
	for i := range chainID {
		chainID[i] = b
	}
	return isc.NewContractAgentID(chainID, isc.Hname(h))
}

var dummyAssetID = [iotago.NativeTokenIDLength]byte{1, 2, 3}

func checkLedgerT(t *testing.T, s *accounts.StateWriter, checkpoint string) *isc.FungibleTokens {
	t.Log(checkpoint)
	require.NotPanics(t, func() {
		s.CheckLedgerConsistency()
	})
	return s.GetTotalL2FungibleTokens()
}

func newStateContext() accounts.StateContext {
	return accounts.NewStateContext(
		allmigrations.DefaultScheme.LatestSchemaVersion(),
		isc.ChainID{},
		testutil.TokenInfo,
		testutil.L1API,
	)
}

func testCreditDebit1(t *testing.T, v isc.SchemaVersion) {
	store := dict.New()
	state := accounts.NewStateWriter(newStateContext(), store)
	total := checkLedgerT(t, state, "cp0")

	require.True(t, total.Equals(isc.NewEmptyFungibleTokens()))

	agentID1 := knownAgentID(1, 2)
	transfer := isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	state.CreditToAccount(agentID1, transfer)
	total = checkLedgerT(t, state, "cp1")

	require.NotNil(t, total)
	require.EqualValues(t, 1, len(total.NativeTokens))
	require.True(t, total.Equals(transfer))

	transfer.BaseTokens = 1
	state.CreditToAccount(agentID1, transfer)
	total = checkLedgerT(t, state, "cp2")

	expected := isc.NewFungibleTokens(43, nil).AddNativeTokens(dummyAssetID, big.NewInt(4))
	require.True(t, expected.Equals(total))

	userAssets := state.GetAccountFungibleTokensDiscardExtraDecimals(agentID1)
	require.EqualValues(t, 43, userAssets.BaseTokens)
	require.Zero(t, userAssets.NativeTokens[dummyAssetID].Cmp(big.NewInt(4)))
	checkLedgerT(t, state, "cp2")

	state.DebitFromAccount(agentID1, expected)
	total = checkLedgerT(t, state, "cp3")
	expected = isc.NewEmptyFungibleTokens()
	require.True(t, expected.Equals(total))
}

func testCreditDebit2(t *testing.T, v isc.SchemaVersion) {
	store := dict.New()
	state := accounts.NewStateWriter(newStateContext(), store)
	total := checkLedgerT(t, state, "cp0")
	require.True(t, total.Equals(isc.NewEmptyFungibleTokens()))

	agentID1 := isc.NewRandomAgentID()
	transfer := isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	state.CreditToAccount(agentID1, transfer)
	total = checkLedgerT(t, state, "cp1")

	expected := transfer
	require.EqualValues(t, 1, len(total.NativeTokens))
	require.True(t, expected.Equals(total))

	transfer = isc.NewEmptyFungibleTokens().AddNativeTokens(dummyAssetID, big.NewInt(2))
	state.DebitFromAccount(agentID1, transfer)
	total = checkLedgerT(t, state, "cp2")
	require.EqualValues(t, 0, len(total.NativeTokens))
	expected = isc.NewFungibleTokens(42, nil)
	require.True(t, expected.Equals(total))

	require.True(t, util.IsZeroBigInt(state.GetNativeTokenBalance(agentID1, dummyAssetID)))
	bal1 := state.GetAccountFungibleTokensDiscardExtraDecimals(agentID1)
	require.False(t, bal1.IsEmpty())
	require.True(t, total.Equals(bal1))
}

func testCreditDebit3(t *testing.T, v isc.SchemaVersion) {
	store := dict.New()
	state := accounts.NewStateWriter(newStateContext(), store)
	total := checkLedgerT(t, state, "cp0")
	require.True(t, total.Equals(isc.NewEmptyFungibleTokens()))

	agentID1 := isc.NewRandomAgentID()
	transfer := isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	state.CreditToAccount(agentID1, transfer)
	total = checkLedgerT(t, state, "cp1")

	expected := transfer
	require.EqualValues(t, 1, len(total.NativeTokens))
	require.True(t, expected.Equals(total))

	transfer = isc.NewEmptyFungibleTokens().AddNativeTokens(dummyAssetID, big.NewInt(100))
	require.Panics(t,
		func() {
			state.DebitFromAccount(agentID1, transfer)
		},
	)
	total = checkLedgerT(t, state, "cp2")

	require.EqualValues(t, 1, len(total.NativeTokens))
	expected = isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	require.True(t, expected.Equals(total))
}

func testCreditDebit4(t *testing.T, v isc.SchemaVersion) {
	store := dict.New()
	state := accounts.NewStateWriter(newStateContext(), store)
	total := checkLedgerT(t, state, "cp0")
	require.True(t, total.Equals(isc.NewEmptyFungibleTokens()))

	agentID1 := isc.NewRandomAgentID()
	transfer := isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	state.CreditToAccount(agentID1, transfer)
	total = checkLedgerT(t, state, "cp1")

	expected := transfer
	require.EqualValues(t, 1, len(total.NativeTokens))
	require.True(t, expected.Equals(total))

	keys := state.AllAccountsAsDict().Keys()
	require.EqualValues(t, 1, len(keys))

	agentID2 := isc.NewRandomAgentID()
	require.NotEqualValues(t, agentID1, agentID2)

	transfer = isc.NewFungibleTokens(20, nil)
	lo.Must0(state.MoveBetweenAccounts(agentID1, agentID2, transfer.ToAssets()))
	total = checkLedgerT(t, state, "cp2")

	keys = state.AllAccountsAsDict().Keys()
	require.EqualValues(t, 2, len(keys))

	expected = isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	require.True(t, expected.Equals(total))

	bm1 := state.GetAccountFungibleTokensDiscardExtraDecimals(agentID1)
	require.False(t, bm1.IsEmpty())
	expected = isc.NewFungibleTokens(22, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	require.True(t, expected.Equals(bm1))

	bm2 := state.GetAccountFungibleTokensDiscardExtraDecimals(agentID2)
	require.False(t, bm2.IsEmpty())
	expected = isc.NewFungibleTokens(20, nil)
	require.True(t, expected.Equals(bm2))
}

func testCreditDebit5(t *testing.T, v isc.SchemaVersion) {
	store := dict.New()
	state := accounts.NewStateWriter(newStateContext(), store)
	total := checkLedgerT(t, state, "cp0")
	require.True(t, total.Equals(isc.NewEmptyFungibleTokens()))

	agentID1 := isc.NewRandomAgentID()
	transfer := isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	state.CreditToAccount(agentID1, transfer)
	total = checkLedgerT(t, state, "cp1")

	expected := transfer
	require.EqualValues(t, 1, len(total.NativeTokens))
	require.True(t, expected.Equals(total))

	keys := state.AllAccountsAsDict().Keys()
	require.EqualValues(t, 1, len(keys))

	agentID2 := isc.NewRandomAgentID()
	require.NotEqualValues(t, agentID1, agentID2)

	transfer = isc.NewFungibleTokens(50, nil)
	require.Error(t, state.MoveBetweenAccounts(agentID1, agentID2, transfer.ToAssets()))
	total = checkLedgerT(t, state, "cp2")

	keys = state.AllAccountsAsDict().Keys()
	require.EqualValues(t, 1, len(keys))

	expected = isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	require.True(t, expected.Equals(total))

	bm1 := state.GetAccountFungibleTokensDiscardExtraDecimals(agentID1)
	require.False(t, bm1.IsEmpty())
	require.True(t, expected.Equals(bm1))

	bm2 := state.GetAccountFungibleTokensDiscardExtraDecimals(agentID2)
	require.True(t, bm2.IsEmpty())
}

func testCreditDebit6(t *testing.T, v isc.SchemaVersion) {
	store := dict.New()
	state := accounts.NewStateWriter(newStateContext(), store)
	total := checkLedgerT(t, state, "cp0")
	require.True(t, total.Equals(isc.NewEmptyFungibleTokens()))

	agentID1 := isc.NewRandomAgentID()
	transfer := isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	state.CreditToAccount(agentID1, transfer)
	checkLedgerT(t, state, "cp1")

	agentID2 := isc.NewRandomAgentID()
	require.NotEqualValues(t, agentID1, agentID2)

	lo.Must0(state.MoveBetweenAccounts(agentID1, agentID2, transfer.ToAssets()))
	total = checkLedgerT(t, state, "cp2")

	keys := state.AllAccountsAsDict().Keys()
	require.EqualValues(t, 2, len(keys))

	bal := state.GetAccountFungibleTokensDiscardExtraDecimals(agentID1)
	require.True(t, bal.IsEmpty())

	bal2 := state.GetAccountFungibleTokensDiscardExtraDecimals(agentID2)
	require.False(t, bal2.IsEmpty())
	require.True(t, total.Equals(bal2))
}

func testCreditDebit7(t *testing.T, v isc.SchemaVersion) {
	store := dict.New()
	state := accounts.NewStateWriter(newStateContext(), store)
	total := checkLedgerT(t, state, "cp0")
	require.True(t, total.Equals(isc.NewEmptyFungibleTokens()))

	agentID1 := isc.NewRandomAgentID()
	transfer := isc.NewEmptyFungibleTokens().AddNativeTokens(dummyAssetID, big.NewInt(2))
	state.CreditToAccount(agentID1, transfer)
	checkLedgerT(t, state, "cp1")

	debitTransfer := isc.NewFungibleTokens(1, nil)
	// debit must fail
	require.Panics(t, func() {
		state.DebitFromAccount(agentID1, debitTransfer)
	})

	total = checkLedgerT(t, state, "cp1")
	require.True(t, transfer.Equals(total))
}

func testMoveAll(t *testing.T, v isc.SchemaVersion) {
	store := dict.New()
	state := accounts.NewStateWriter(newStateContext(), store)
	agentID1 := isc.NewRandomAgentID()
	agentID2 := isc.NewRandomAgentID()

	transfer := isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	state.CreditToAccount(agentID1, transfer)
	require.EqualValues(t, 1, state.AllAccountsMap().Len())
	accs := state.AllAccountsAsDict()
	require.EqualValues(t, 1, len(accs))
	_, ok := accs[kv.Key(agentID1.Bytes())]
	require.True(t, ok)

	lo.Must0(state.MoveBetweenAccounts(agentID1, agentID2, transfer.ToAssets()))
	require.EqualValues(t, 2, state.AllAccountsMap().Len())
	accs = state.AllAccountsAsDict()
	require.EqualValues(t, 2, len(accs))
	_, ok = accs[kv.Key(agentID2.Bytes())]
	require.True(t, ok)
}

func testDebitAll(t *testing.T, v isc.SchemaVersion) {
	store := dict.New()
	state := accounts.NewStateWriter(newStateContext(), store)
	agentID1 := isc.NewRandomAgentID()

	transfer := isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	state.CreditToAccount(agentID1, transfer)
	require.EqualValues(t, 1, state.AllAccountsMap().Len())
	accs := state.AllAccountsAsDict()
	require.EqualValues(t, 1, len(accs))
	_, ok := accs[kv.Key(agentID1.Bytes())]
	require.True(t, ok)

	state.DebitFromAccount(agentID1, transfer)
	require.EqualValues(t, 1, state.AllAccountsMap().Len())
	accs = state.AllAccountsAsDict()
	require.EqualValues(t, 1, len(accs))
	require.True(t, ok)

	assets := state.GetAccountFungibleTokensDiscardExtraDecimals(agentID1)
	require.True(t, assets.IsEmpty())

	assets = state.GetTotalL2FungibleTokens()
	require.True(t, assets.IsEmpty())
}

func testTransferNFTs(t *testing.T, v isc.SchemaVersion) {
	store := dict.New()
	state := accounts.NewStateWriter(newStateContext(), store)
	total := checkLedgerT(t, state, "cp0")

	require.True(t, total.Equals(isc.NewEmptyFungibleTokens()))

	agentID1 := isc.NewRandomAgentID()
	NFT1 := &isc.NFT{
		ID:       iotago.NFTID{123},
		Issuer:   tpkg.RandEd25519Address(),
		Metadata: iotago.MetadataFeatureEntries{"": []byte("foobar")},
	}
	state.CreditNFTToAccount(agentID1, &iotago.NFTOutput{
		Amount: 0,
		NFTID:  NFT1.ID,
		ImmutableFeatures: iotago.NFTOutputImmFeatures{
			&iotago.IssuerFeature{Address: NFT1.Issuer},
			&iotago.MetadataFeature{Entries: NFT1.Metadata},
		},
	})
	// nft is credited
	user1NFTs := state.GetAccountNFTs(agentID1)
	require.Len(t, user1NFTs, 1)
	require.Equal(t, user1NFTs[0], NFT1.ID)

	// nft data is saved (state.SaveNFTOutput must be called)
	state.SaveNFTOutput(&iotago.NFTOutput{
		Amount: 0,
		NFTID:  NFT1.ID,
		ImmutableFeatures: iotago.NFTOutputImmFeatures{
			&iotago.IssuerFeature{Address: NFT1.Issuer},
			&iotago.MetadataFeature{Entries: NFT1.Metadata},
		},
	}, 0)

	nftData := state.GetNFTData(NFT1.ID)
	require.Equal(t, nftData.ID, NFT1.ID)
	require.Equal(t, nftData.Issuer, NFT1.Issuer)
	require.Equal(t, nftData.Metadata, NFT1.Metadata)

	agentID2 := isc.NewRandomAgentID()

	// cannot move an NFT that is not owned
	require.Error(t, state.MoveBetweenAccounts(agentID1, agentID2, isc.NewEmptyAssets().AddNFTs(iotago.NFTID{111})))

	// moves successfully when the NFT is owned
	lo.Must0(state.MoveBetweenAccounts(agentID1, agentID2, isc.NewEmptyAssets().AddNFTs(NFT1.ID)))

	user1NFTs = state.GetAccountNFTs(agentID1)
	require.Len(t, user1NFTs, 0)
	user2NFTs := state.GetAccountNFTs(agentID2)
	require.Len(t, user2NFTs, 1)
	require.Equal(t, user2NFTs[0], NFT1.ID)

	// remove the NFT from the chain
	state.DebitNFTFromAccount(agentID2, NFT1.ID)
	require.Panics(t, func() {
		state.GetNFTData(NFT1.ID)
	})
}

func testCreditDebitNFT1(t *testing.T, _ isc.SchemaVersion) {
	store := dict.New()
	state := accounts.NewStateWriter(newStateContext(), store)

	agentID1 := knownAgentID(1, 2)
	nft := isc.NFT{
		ID:       iotago.NFTID{123},
		Issuer:   tpkg.RandEd25519Address(),
		Metadata: iotago.MetadataFeatureEntries{"": []byte("foobar")},
	}
	state.CreditNFTToAccount(agentID1, &iotago.NFTOutput{
		Amount: 0,
		NFTID:  nft.ID,
		ImmutableFeatures: iotago.NFTOutputImmFeatures{
			&iotago.IssuerFeature{Address: nft.Issuer},
			&iotago.MetadataFeature{Entries: nft.Metadata},
		},
	})

	accNFTs := state.GetAccountNFTs(agentID1)
	require.Len(t, accNFTs, 1)
	require.Equal(t, accNFTs[0], nft.ID)

	state.DebitNFTFromAccount(agentID1, nft.ID)

	accNFTs = state.GetAccountNFTs(agentID1)
	require.Len(t, accNFTs, 0)
}
