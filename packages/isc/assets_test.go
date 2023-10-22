package isc_test

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/tpkg"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

func TestAssetsSerialization(t *testing.T) {
	maxVal, e := big.NewInt(2), big.NewInt(256)
	maxVal.Exp(maxVal, e, nil)
	maxVal.Sub(maxVal, big.NewInt(1))

	tokens := []*iotago.NativeTokenFeature{
		{
			ID:     [iotago.NativeTokenIDLength]byte{1},
			Amount: big.NewInt(100),
		},
		{
			ID:     [iotago.NativeTokenIDLength]byte{2},
			Amount: big.NewInt(200),
		},
		{
			ID:     [iotago.NativeTokenIDLength]byte{3},
			Amount: util.MaxUint256,
		},
	}

	assets := isc.NewAssets(1, tokens)
	rwutil.BytesTest(t, assets, isc.AssetsFromBytes)
}

func TestAssetsSpendBudget(t *testing.T) {
	var toSpend *isc.Assets
	var budget *isc.Assets
	require.True(t, budget.Spend(toSpend))
	require.True(t, budget.IsEmpty())
	require.True(t, budget.IsEmpty())

	budget = &isc.Assets{BaseTokens: 1}
	require.True(t, budget.Spend(toSpend))
	require.False(t, toSpend.Spend(budget))

	budget = &isc.Assets{BaseTokens: 10}
	require.True(t, budget.Spend(budget))
	require.True(t, budget.IsEmpty())

	budget = &isc.Assets{BaseTokens: 2}
	toSpend = &isc.Assets{BaseTokens: 1}
	require.True(t, budget.Spend(toSpend))
	require.True(t, budget.Equals(&isc.Assets{
		BaseTokens:   1,
		NativeTokens: []*iotago.NativeTokenFeature{},
		NFTs:         []iotago.NFTID{},
	}))

	budget = &isc.Assets{BaseTokens: 1}
	toSpend = &isc.Assets{BaseTokens: 2}
	require.False(t, budget.Spend(toSpend))
	require.True(t, budget.Equals(&isc.Assets{
		BaseTokens:   1,
		NativeTokens: []*iotago.NativeTokenFeature{},
		NFTs:         []iotago.NFTID{},
	}))

	nativeTokenID1 := tpkg.RandNativeTokenFeature().ID
	nativeTokenID2 := tpkg.RandNativeTokenFeature().ID

	budget = &isc.Assets{
		BaseTokens: 1,
		NativeTokens: []*iotago.NativeTokenFeature{
			{ID: nativeTokenID1, Amount: big.NewInt(5)},
		},
	}
	toSpend = budget.Clone()
	require.True(t, budget.Spend(toSpend))
	println(budget.String())
	require.True(t, budget.IsEmpty())

	budget = &isc.Assets{
		BaseTokens: 1,
		NativeTokens: []*iotago.NativeTokenFeature{
			{ID: nativeTokenID1, Amount: big.NewInt(5)},
		},
	}
	cloneBudget := budget.Clone()
	toSpend = &isc.Assets{
		BaseTokens: 1,
		NativeTokens: []*iotago.NativeTokenFeature{
			{ID: nativeTokenID1, Amount: big.NewInt(10)},
		},
	}
	require.False(t, budget.Spend(toSpend))
	require.True(t, budget.Equals(cloneBudget))

	budget = &isc.Assets{
		BaseTokens: 1,
		NativeTokens: []*iotago.NativeTokenFeature{
			{ID: nativeTokenID1, Amount: big.NewInt(5)},
			{ID: nativeTokenID2, Amount: big.NewInt(1)},
		},
	}
	toSpend = &isc.Assets{
		BaseTokens: 1,
		NativeTokens: []*iotago.NativeTokenFeature{
			{ID: nativeTokenID1, Amount: big.NewInt(5)},
		},
	}
	expected := &isc.Assets{
		BaseTokens: 0,
		NativeTokens: []*iotago.NativeTokenFeature{
			{ID: nativeTokenID2, Amount: big.NewInt(1)},
		},
	}
	require.True(t, budget.Spend(toSpend))
	require.True(t, budget.Equals(expected))

	budget = &isc.Assets{
		BaseTokens: 10,
		NativeTokens: []*iotago.NativeTokenFeature{
			{ID: nativeTokenID2, Amount: big.NewInt(1)},
		},
	}
	toSpend = &isc.Assets{
		BaseTokens: 1,
		NativeTokens: []*iotago.NativeTokenFeature{
			{ID: nativeTokenID1, Amount: big.NewInt(5)},
		},
	}

	require.False(t, budget.Spend(toSpend))
}

func TestAssetsAddNFTs(t *testing.T) {
	nftSet1 := []iotago.NFTID{
		{1},
		{2},
		{3},
	}

	nftSet2 := []iotago.NFTID{
		{3},
		{4},
		{5},
	}
	a := isc.NewAssets(0, nil, nftSet1...)
	b := isc.NewAssets(0, nil, nftSet2...)
	a.Add(b)
	require.Len(t, a.NFTs, 5)
	require.Contains(t, a.NFTs, iotago.NFTID{1})
	require.Contains(t, a.NFTs, iotago.NFTID{2})
	require.Contains(t, a.NFTs, iotago.NFTID{3})
	require.Contains(t, a.NFTs, iotago.NFTID{4})
	require.Contains(t, a.NFTs, iotago.NFTID{5})
}
