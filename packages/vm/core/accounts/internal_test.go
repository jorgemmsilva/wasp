package accounts

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
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

func knownAgentID(b byte, h uint32) isc.AgentID {
	var chainID isc.ChainID
	for i := range chainID {
		chainID[i] = b
	}
	return isc.NewContractAgentID(chainID, isc.Hname(h))
}

func TestBasic(t *testing.T) {
	t.Logf("Name: %s", Contract.Name)
	t.Logf("Program hash: %s", Contract.ProgramHash.String())
	t.Logf("Hname: %s", Contract.Hname())
}

var dummyAssetID = [iotago.NativeTokenIDLength]byte{1, 2, 3}

func checkLedgerT(t *testing.T, state dict.Dict, cp string) *isc.FungibleTokens {
	require.NotPanics(t, func() {
		CheckLedger(state, cp, testutil.TokenInfo)
	})
	return GetTotalL2FungibleTokens(state, testutil.TokenInfo)
}

func TestCreditDebit1(t *testing.T) {
	state := dict.New()
	total := checkLedgerT(t, state, "cp0")

	require.True(t, total.Equals(isc.NewEmptyFungibleTokens()))

	agentID1 := knownAgentID(1, 2)
	transfer := isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	CreditToAccount(state, agentID1, transfer, isc.ChainID{}, testutil.TokenInfo)
	total = checkLedgerT(t, state, "cp1")

	require.NotNil(t, total)
	require.EqualValues(t, 1, len(total.NativeTokens))
	require.True(t, total.Equals(transfer))

	transfer.BaseTokens = 1
	CreditToAccount(state, agentID1, transfer, isc.ChainID{}, testutil.TokenInfo)
	total = checkLedgerT(t, state, "cp2")

	expected := isc.NewFungibleTokens(43, nil).AddNativeTokens(dummyAssetID, big.NewInt(4))
	require.True(t, expected.Equals(total))

	userAssets := GetAccountFungibleTokens(state, agentID1, isc.ChainID{}, testutil.TokenInfo)
	require.EqualValues(t, 43, userAssets.BaseTokens)
	require.Zero(t, userAssets.NativeTokens[dummyAssetID].Cmp(big.NewInt(4)))
	checkLedgerT(t, state, "cp2")

	DebitFromAccount(state, agentID1, expected, isc.ChainID{}, testutil.TokenInfo)
	total = checkLedgerT(t, state, "cp3")
	expected = isc.NewEmptyFungibleTokens()
	require.True(t, expected.Equals(total))
}

func TestCreditDebit2(t *testing.T) {
	state := dict.New()
	total := checkLedgerT(t, state, "cp0")
	require.True(t, total.Equals(isc.NewEmptyFungibleTokens()))

	agentID1 := isc.NewRandomAgentID()
	transfer := isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	CreditToAccount(state, agentID1, transfer, isc.ChainID{}, testutil.TokenInfo)
	total = checkLedgerT(t, state, "cp1")

	expected := transfer
	require.EqualValues(t, 1, len(total.NativeTokens))
	require.True(t, expected.Equals(total))

	transfer = isc.NewEmptyFungibleTokens().AddNativeTokens(dummyAssetID, big.NewInt(2))
	DebitFromAccount(state, agentID1, transfer, isc.ChainID{}, testutil.TokenInfo)
	total = checkLedgerT(t, state, "cp2")
	require.EqualValues(t, 0, len(total.NativeTokens))
	expected = isc.NewFungibleTokens(42, nil)
	require.True(t, expected.Equals(total))

	require.True(t, util.IsZeroBigInt(GetNativeTokenBalance(state, agentID1, dummyAssetID, isc.ChainID{})))
	bal1 := GetAccountFungibleTokens(state, agentID1, isc.ChainID{}, testutil.TokenInfo)
	require.False(t, bal1.IsEmpty())
	require.True(t, total.Equals(bal1))
}

func TestCreditDebit3(t *testing.T) {
	state := dict.New()
	total := checkLedgerT(t, state, "cp0")
	require.True(t, total.Equals(isc.NewEmptyFungibleTokens()))

	agentID1 := isc.NewRandomAgentID()
	transfer := isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	CreditToAccount(state, agentID1, transfer, isc.ChainID{}, testutil.TokenInfo)
	total = checkLedgerT(t, state, "cp1")

	expected := transfer
	require.EqualValues(t, 1, len(total.NativeTokens))
	require.True(t, expected.Equals(total))

	transfer = isc.NewEmptyFungibleTokens().AddNativeTokens(dummyAssetID, big.NewInt(100))
	require.Panics(t,
		func() {
			DebitFromAccount(state, agentID1, transfer, isc.ChainID{}, testutil.TokenInfo)
		},
	)
	total = checkLedgerT(t, state, "cp2")

	require.EqualValues(t, 1, len(total.NativeTokens))
	expected = isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	require.True(t, expected.Equals(total))
}

func TestCreditDebit4(t *testing.T) {
	state := dict.New()
	total := checkLedgerT(t, state, "cp0")
	require.True(t, total.Equals(isc.NewEmptyFungibleTokens()))

	agentID1 := isc.NewRandomAgentID()
	transfer := isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	CreditToAccount(state, agentID1, transfer, isc.ChainID{}, testutil.TokenInfo)
	total = checkLedgerT(t, state, "cp1")

	expected := transfer
	require.EqualValues(t, 1, len(total.NativeTokens))
	require.True(t, expected.Equals(total))

	keys := allAccountsAsDict(state).Keys()
	require.EqualValues(t, 1, len(keys))

	agentID2 := isc.NewRandomAgentID()
	require.NotEqualValues(t, agentID1, agentID2)

	transfer = isc.NewFungibleTokens(20, nil)
	lo.Must0(MoveBetweenAccounts(state, agentID1, agentID2, transfer.ToAssets(), isc.ChainID{}, testutil.TokenInfo))
	total = checkLedgerT(t, state, "cp2")

	keys = allAccountsAsDict(state).Keys()
	require.EqualValues(t, 2, len(keys))

	expected = isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	require.True(t, expected.Equals(total))

	bm1 := GetAccountFungibleTokens(state, agentID1, isc.ChainID{}, testutil.TokenInfo)
	require.False(t, bm1.IsEmpty())
	expected = isc.NewFungibleTokens(22, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	require.True(t, expected.Equals(bm1))

	bm2 := GetAccountFungibleTokens(state, agentID2, isc.ChainID{}, testutil.TokenInfo)
	require.False(t, bm2.IsEmpty())
	expected = isc.NewFungibleTokens(20, nil)
	require.True(t, expected.Equals(bm2))
}

func TestCreditDebit5(t *testing.T) {
	state := dict.New()
	total := checkLedgerT(t, state, "cp0")
	require.True(t, total.Equals(isc.NewEmptyFungibleTokens()))

	agentID1 := isc.NewRandomAgentID()
	transfer := isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	CreditToAccount(state, agentID1, transfer, isc.ChainID{}, testutil.TokenInfo)
	total = checkLedgerT(t, state, "cp1")

	expected := transfer
	require.EqualValues(t, 1, len(total.NativeTokens))
	require.True(t, expected.Equals(total))

	keys := allAccountsAsDict(state).Keys()
	require.EqualValues(t, 1, len(keys))

	agentID2 := isc.NewRandomAgentID()
	require.NotEqualValues(t, agentID1, agentID2)

	transfer = isc.NewFungibleTokens(50, nil)
	require.Error(t, MoveBetweenAccounts(state, agentID1, agentID2, transfer.ToAssets(), isc.ChainID{}, testutil.TokenInfo))
	total = checkLedgerT(t, state, "cp2")

	keys = allAccountsAsDict(state).Keys()
	require.EqualValues(t, 1, len(keys))

	expected = isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	require.True(t, expected.Equals(total))

	bm1 := GetAccountFungibleTokens(state, agentID1, isc.ChainID{}, testutil.TokenInfo)
	require.False(t, bm1.IsEmpty())
	require.True(t, expected.Equals(bm1))

	bm2 := GetAccountFungibleTokens(state, agentID2, isc.ChainID{}, testutil.TokenInfo)
	require.True(t, bm2.IsEmpty())
}

func TestCreditDebit6(t *testing.T) {
	state := dict.New()
	total := checkLedgerT(t, state, "cp0")
	require.True(t, total.Equals(isc.NewEmptyFungibleTokens()))

	agentID1 := isc.NewRandomAgentID()
	transfer := isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	CreditToAccount(state, agentID1, transfer, isc.ChainID{}, testutil.TokenInfo)
	checkLedgerT(t, state, "cp1")

	agentID2 := isc.NewRandomAgentID()
	require.NotEqualValues(t, agentID1, agentID2)

	lo.Must0(MoveBetweenAccounts(state, agentID1, agentID2, transfer.ToAssets(), isc.ChainID{}, testutil.TokenInfo))
	total = checkLedgerT(t, state, "cp2")

	keys := allAccountsAsDict(state).Keys()
	require.EqualValues(t, 2, len(keys))

	bal := GetAccountFungibleTokens(state, agentID1, isc.ChainID{}, testutil.TokenInfo)
	require.True(t, bal.IsEmpty())

	bal2 := GetAccountFungibleTokens(state, agentID2, isc.ChainID{}, testutil.TokenInfo)
	require.False(t, bal2.IsEmpty())
	require.True(t, total.Equals(bal2))
}

func TestCreditDebit7(t *testing.T) {
	state := dict.New()
	total := checkLedgerT(t, state, "cp0")
	require.True(t, total.Equals(isc.NewEmptyFungibleTokens()))

	agentID1 := isc.NewRandomAgentID()
	transfer := isc.NewEmptyFungibleTokens().AddNativeTokens(dummyAssetID, big.NewInt(2))
	CreditToAccount(state, agentID1, transfer, isc.ChainID{}, testutil.TokenInfo)
	checkLedgerT(t, state, "cp1")

	debitTransfer := isc.NewFungibleTokens(1, nil)
	// debit must fail
	require.Panics(t, func() {
		DebitFromAccount(state, agentID1, debitTransfer, isc.ChainID{}, testutil.TokenInfo)
	})

	total = checkLedgerT(t, state, "cp1")
	require.True(t, transfer.Equals(total))
}

func TestMoveAll(t *testing.T) {
	state := dict.New()
	agentID1 := isc.NewRandomAgentID()
	agentID2 := isc.NewRandomAgentID()

	transfer := isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	CreditToAccount(state, agentID1, transfer, isc.ChainID{}, testutil.TokenInfo)
	require.EqualValues(t, 1, allAccountsMapR(state).Len())
	accs := allAccountsAsDict(state)
	require.EqualValues(t, 1, len(accs))
	_, ok := accs[kv.Key(agentID1.Bytes())]
	require.True(t, ok)

	lo.Must0(MoveBetweenAccounts(state, agentID1, agentID2, transfer.ToAssets(), isc.ChainID{}, testutil.TokenInfo))
	require.EqualValues(t, 2, allAccountsMapR(state).Len())
	accs = allAccountsAsDict(state)
	require.EqualValues(t, 2, len(accs))
	_, ok = accs[kv.Key(agentID2.Bytes())]
	require.True(t, ok)
}

func TestDebitAll(t *testing.T) {
	state := dict.New()
	agentID1 := isc.NewRandomAgentID()

	transfer := isc.NewFungibleTokens(42, nil).AddNativeTokens(dummyAssetID, big.NewInt(2))
	CreditToAccount(state, agentID1, transfer, isc.ChainID{}, testutil.TokenInfo)
	require.EqualValues(t, 1, allAccountsMapR(state).Len())
	accs := allAccountsAsDict(state)
	require.EqualValues(t, 1, len(accs))
	_, ok := accs[kv.Key(agentID1.Bytes())]
	require.True(t, ok)

	DebitFromAccount(state, agentID1, transfer, isc.ChainID{}, testutil.TokenInfo)
	require.EqualValues(t, 1, allAccountsMapR(state).Len())
	accs = allAccountsAsDict(state)
	require.EqualValues(t, 1, len(accs))
	require.True(t, ok)

	assets := GetAccountFungibleTokens(state, agentID1, isc.ChainID{}, testutil.TokenInfo)
	require.True(t, assets.IsEmpty())

	assets = GetTotalL2FungibleTokens(state, testutil.TokenInfo)
	require.True(t, assets.IsEmpty())
}

func TestTransferNFTs(t *testing.T) {
	state := dict.New()
	total := checkLedgerT(t, state, "cp0")

	require.True(t, total.Equals(isc.NewEmptyFungibleTokens()))

	agentID1 := isc.NewRandomAgentID()
	NFT1 := &isc.NFT{
		ID:       iotago.NFTID{123},
		Issuer:   tpkg.RandEd25519Address(),
		Metadata: iotago.MetadataFeatureEntries{"": []byte("foobar")},
	}
	CreditNFTToAccount(state, agentID1, &iotago.NFTOutput{
		Amount: 0,
		NFTID:  NFT1.ID,
		ImmutableFeatures: []iotago.Feature{
			&iotago.IssuerFeature{Address: NFT1.Issuer},
			&iotago.MetadataFeature{Entries: NFT1.Metadata},
		},
	}, isc.ChainID{})
	// nft is credited
	user1NFTs := getAccountNFTs(state, agentID1)
	require.Len(t, user1NFTs, 1)
	require.Equal(t, user1NFTs[0], NFT1.ID)

	// nft data is saved (accounts.SaveNFTOutput must be called)
	SaveNFTOutput(state, &iotago.NFTOutput{
		Amount: 0,
		NFTID:  NFT1.ID,
		ImmutableFeatures: []iotago.Feature{
			&iotago.IssuerFeature{Address: NFT1.Issuer},
			&iotago.MetadataFeature{Entries: NFT1.Metadata},
		},
	}, 0)

	nftData := GetNFTData(state, NFT1.ID)
	require.Equal(t, nftData.ID, NFT1.ID)
	require.Equal(t, nftData.Issuer, NFT1.Issuer)
	require.Equal(t, nftData.Metadata, NFT1.Metadata)

	agentID2 := isc.NewRandomAgentID()

	// cannot move an NFT that is not owned
	require.Error(t, MoveBetweenAccounts(state, agentID1, agentID2, isc.NewEmptyAssets().AddNFTs(iotago.NFTID{111}), isc.ChainID{}, testutil.TokenInfo))

	// moves successfully when the NFT is owned
	lo.Must0(MoveBetweenAccounts(state, agentID1, agentID2, isc.NewEmptyAssets().AddNFTs(NFT1.ID), isc.ChainID{}, testutil.TokenInfo))

	user1NFTs = getAccountNFTs(state, agentID1)
	require.Len(t, user1NFTs, 0)
	user2NFTs := getAccountNFTs(state, agentID2)
	require.Len(t, user2NFTs, 1)
	require.Equal(t, user2NFTs[0], NFT1.ID)

	// remove the NFT from the chain
	DebitNFTFromAccount(state, agentID2, NFT1.ID, isc.ChainID{})
	require.Panics(t, func() {
		GetNFTData(state, NFT1.ID)
	})
}

func TestFoundryOutputRecSerialization(t *testing.T) {
	o := foundryOutputRec{
		OutputID: iotago.OutputID{1, 2, 3},
		Amount:   300,
		TokenScheme: &iotago.SimpleTokenScheme{
			MaximumSupply: big.NewInt(1000),
			MintedTokens:  big.NewInt(20),
			MeltedTokens:  big.NewInt(1),
		},
		Metadata: []byte("Tralala"),
	}
	rwutil.ReadWriteTest(t, &o, new(foundryOutputRec))
	rwutil.BytesTest(t, &o, foundryOutputRecFromBytes)
}

func TestCreditDebitNFT1(t *testing.T) {
	state := dict.New()

	agentID1 := knownAgentID(1, 2)
	nft := isc.NFT{
		ID:       iotago.NFTID{123},
		Issuer:   tpkg.RandEd25519Address(),
		Metadata: iotago.MetadataFeatureEntries{"": []byte("foobar")},
	}
	CreditNFTToAccount(state, agentID1, &iotago.NFTOutput{
		Amount: 0,
		NFTID:  nft.ID,
		ImmutableFeatures: []iotago.Feature{
			&iotago.IssuerFeature{Address: nft.Issuer},
			&iotago.MetadataFeature{Entries: nft.Metadata},
		},
	}, isc.ChainID{})

	accNFTs := GetAccountNFTs(state, agentID1)
	require.Len(t, accNFTs, 1)
	require.Equal(t, accNFTs[0], nft.ID)

	DebitNFTFromAccount(state, agentID1, nft.ID, isc.ChainID{})

	accNFTs = GetAccountNFTs(state, agentID1)
	require.Len(t, accNFTs, 0)
}
