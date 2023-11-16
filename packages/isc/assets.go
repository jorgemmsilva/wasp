package isc

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"math/big"
	"slices"
	"sort"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

var BaseTokenID = []byte{}

// IsBaseToken return whether a given tokenID represents the base token
func IsBaseToken(tokenID []byte) bool {
	return bytes.Equal(tokenID, BaseTokenID)
}

type FungibleTokens struct {
	BaseTokens   iotago.BaseToken
	NativeTokens iotago.NativeTokenSum
}

func NewFungibleTokens(baseTokens iotago.BaseToken, tokens iotago.NativeTokenSum) *FungibleTokens {
	f := &FungibleTokens{
		BaseTokens:   baseTokens,
		NativeTokens: iotago.NativeTokenSum{},
	}
	for id, n := range tokens {
		f.AddNativeTokens(id, n)
	}
	return f
}

func NewEmptyFungibleTokens() *FungibleTokens {
	return NewFungibleTokens(0, nil)
}

func (f *FungibleTokens) AddBaseTokens(amount iotago.BaseToken) *FungibleTokens {
	f.BaseTokens += amount
	return f
}

func (f *FungibleTokens) AddNativeTokens(nativeTokenID iotago.NativeTokenID, amount *big.Int) *FungibleTokens {
	if n, ok := f.NativeTokens[nativeTokenID]; ok {
		n.Add(amount, amount)
	} else {
		f.NativeTokens[nativeTokenID] = new(big.Int).Set(amount)
	}
	return f
}

func (f *FungibleTokens) ToAssets() *Assets {
	return NewAssets(f.BaseTokens, f.NativeTokens)
}

func (f *FungibleTokens) Clone() *FungibleTokens {
	return NewFungibleTokens(f.BaseTokens, f.NativeTokens)
}

func (a *FungibleTokens) String() string {
	ret := fmt.Sprintf("base tokens: %d", a.BaseTokens)
	if len(a.NativeTokens) > 0 {
		ret += fmt.Sprintf(", tokens (%d):", len(a.NativeTokens))
	}
	for id, n := range a.NativeTokens {
		ret += fmt.Sprintf("\n       %s: %s", id.String(), n.Text(10))
	}
	return ret
}

func (f *FungibleTokens) Equals(b *FungibleTokens) bool {
	if f == nil || b == nil {
		panic("nil FungibleTokens")
	}
	if f == b {
		return true
	}
	if f.BaseTokens != b.BaseTokens {
		return false
	}
	return maps.EqualFunc(f.NativeTokens, b.NativeTokens, func(nt1, nt2 *big.Int) bool {
		return nt1.Cmp(nt2) == 0
	})
}

func (f *FungibleTokens) Geq(b *FungibleTokens) bool {
	if f.IsEmpty() {
		return b.IsEmpty()
	}
	if b.IsEmpty() {
		return true
	}
	if f.BaseTokens < b.BaseTokens {
		return false
	}
	for id, bAmount := range b.NativeTokens {
		fAmount, ok := f.NativeTokens[id]
		if !ok || fAmount.Cmp(bAmount) < 0 {
			return false
		}
	}
	return true
}

// Spend subtracts tokens from the current set.
// Mutates receiver `f` !
// If budget is not enough, returns false and leaves receiver untouched
func (f *FungibleTokens) Spend(toSpend *FungibleTokens) bool {
	if !f.Geq(toSpend) {
		return false
	}
	f.BaseTokens -= toSpend.BaseTokens
	for id, spendAmt := range toSpend.NativeTokens {
		fAmt := f.NativeTokens[id]
		f.NativeTokens[id] = fAmt.Sub(fAmt, spendAmt)
		if f.NativeTokens[id].Sign() == 0 {
			delete(f.NativeTokens, id)
		}
	}
	return true
}

func (f *FungibleTokens) Add(b *FungibleTokens) *FungibleTokens {
	f.BaseTokens += b.BaseTokens
	for id, bAmt := range b.NativeTokens {
		fAmt := f.NativeTokens.ValueOrBigInt0(id)
		f.NativeTokens[id] = fAmt.Add(fAmt, bAmt)
	}
	return f
}

func (f *FungibleTokens) IsEmpty() bool {
	return f.BaseTokens == 0 && len(f.NativeTokens) == 0
}

func (f *FungibleTokens) NativeTokenIDsSorted() []iotago.NativeTokenID {
	ids := lo.Keys(f.NativeTokens)
	sort.Slice(ids, func(lhs, rhs int) bool {
		return bytes.Compare(ids[lhs][:], ids[rhs][:]) < 0
	})
	return ids
}

func (f *FungibleTokens) ToDict() dict.Dict {
	ret := dict.New()
	ret.Set(kv.Key(BaseTokenID), new(big.Int).SetUint64(uint64(f.BaseTokens)).Bytes())
	for id, amt := range f.NativeTokens {
		ret.Set(kv.Key(id[:]), amt.Bytes())
	}
	return ret
}

func FungibleTokensFromDict(d dict.Dict) (*FungibleTokens, error) {
	ret := NewEmptyFungibleTokens()
	for key, val := range d {
		if IsBaseToken([]byte(key)) {
			ret.BaseTokens = iotago.BaseToken(new(big.Int).SetBytes(val).Uint64())
			continue
		}
		id, err := NativeTokenIDFromBytes([]byte(key))
		if err != nil {
			return nil, fmt.Errorf("Assets: %w", err)
		}
		ret.AddNativeTokens(id, new(big.Int).SetBytes(val))
	}
	return ret, nil
}

func FungibleTokensFromOutput(o iotago.Output) *FungibleTokens {
	ret := NewFungibleTokens(o.BaseTokenAmount(), nil)
	if nt := o.FeatureSet().NativeToken(); nt != nil {
		ret.AddNativeTokens(nt.ID, nt.Amount)
	}
	return ret
}

type Assets struct {
	FungibleTokens
	NFTs []iotago.NFTID
}

func NewAssets(baseTokens iotago.BaseToken, tokens iotago.NativeTokenSum, nfts ...iotago.NFTID) *Assets {
	ret := &Assets{FungibleTokens: *NewFungibleTokens(baseTokens, tokens)}
	if len(nfts) != 0 {
		ret.AddNFTs(nfts...)
	}
	return ret
}

func NewAssetsBaseTokens(amount iotago.BaseToken) *Assets {
	return NewAssets(amount, nil)
}

func NewEmptyAssets() *Assets {
	return NewAssets(0, nil)
}

func AssetsFromBytes(b []byte) (*Assets, error) {
	if len(b) == 0 {
		return NewEmptyAssets(), nil
	}
	return rwutil.ReadFromBytes(b, NewEmptyAssets())
}

func AssetsFromOutput(o iotago.Output, oid iotago.OutputID) *Assets {
	ret := &Assets{FungibleTokens: *FungibleTokensFromOutput(o)}
	if o.Type() == iotago.OutputNFT {
		ret.NFTs = []iotago.NFTID{util.NFTIDFromNFTOutput(o.(*iotago.NFTOutput), oid)}
	}
	return ret
}

func AssetsFromOutputMap(outs map[iotago.OutputID]iotago.Output) *Assets {
	ret := NewEmptyAssets()
	for oid, out := range outs {
		ret.Add(AssetsFromOutput(out, oid))
	}
	return ret
}

func (a *Assets) Clone() *Assets {
	return &Assets{
		FungibleTokens: *a.FungibleTokens.Clone(),
		NFTs:           slices.Clone(a.NFTs),
	}
}

func (a *Assets) AddNFTs(nfts ...iotago.NFTID) *Assets {
	a.NFTs = lo.Uniq(append(a.NFTs, nfts...))
	return a
}

func (a *Assets) String() string {
	ret := a.FungibleTokens.String()
	for _, nftid := range a.NFTs {
		ret += fmt.Sprintf("\n NFTID: %s", nftid.String())
	}
	return ret
}

func (a *Assets) Bytes() []byte {
	return rwutil.WriteToBytes(a)
}

func (a *Assets) Equals(b *Assets) bool {
	if a == nil || b == nil {
		panic("nil Assets")
	}
	if a == b {
		return true
	}
	if !a.FungibleTokens.Equals(&b.FungibleTokens) {
		return false
	}
	return maps.Equal(a.NFTSet(), b.NFTSet())
}

func (a *Assets) Geq(b *Assets) bool {
	if a.IsEmpty() {
		return b.IsEmpty()
	}
	if b.IsEmpty() {
		return true
	}
	if !a.FungibleTokens.Geq(&b.FungibleTokens) {
		return false
	}
	return lo.Every(a.NFTs, b.NFTs)
}

// Spend subtracts assets from the current set.
// Mutates receiver `a` !
// If budget is not enough, returns false and leaves receiver untouched
func (a *Assets) Spend(toSpend *Assets) bool {
	if !a.Geq(toSpend) {
		return false
	}
	a.FungibleTokens.Spend(&toSpend.FungibleTokens)
	a.NFTs = lo.Reject(a.NFTs, func(id iotago.NFTID, i int) bool {
		return lo.Contains(toSpend.NFTs, id)
	})
	return true
}

func (a *Assets) NFTSet() map[iotago.NFTID]bool {
	ret := map[iotago.NFTID]bool{}
	for _, nft := range a.NFTs {
		ret[nft] = true
	}
	return ret
}

func (a *Assets) Add(b *Assets) *Assets {
	a.FungibleTokens.Add(&b.FungibleTokens)
	a.AddNFTs(b.NFTs...)
	return a
}

func (a *Assets) IsEmpty() bool {
	return a == nil || a.FungibleTokens.IsEmpty() && len(a.NFTs) == 0
}

func (a *Assets) AddBaseTokens(amount iotago.BaseToken) *Assets {
	a.FungibleTokens.AddBaseTokens(amount)
	return a
}

func (a *Assets) AddNativeTokens(nativeTokenID iotago.NativeTokenID, amount *big.Int) *Assets {
	a.FungibleTokens.AddNativeTokens(nativeTokenID, amount)
	return a
}

func (a *Assets) fillEmptyNFTIDs(output iotago.Output, outputID iotago.OutputID) *Assets {
	nftOutput, ok := output.(*iotago.NFTOutput)
	if !ok {
		return a
	}
	// see if there is an empty NFTID in the assets (this can happen if the NFT is minted as a request to the chain)
	for i, nftID := range a.NFTs {
		if nftID.Empty() {
			a.NFTs[i] = util.NFTIDFromNFTOutput(nftOutput, outputID)
		}
	}
	return a
}

// Since we are encoding an empty assets with a byte already,
// we may as well use more of the byte to compress the data further.
// We're adding 3 flags to indicate the presence of the subcomponents
// of the assets so that we may skip reading/writing them altogether.
const (
	hasBaseTokens   = 0x80
	hasNativeTokens = 0x40
	hasNFTs         = 0x20
)

func (a *Assets) Read(r io.Reader) error {
	rr := rwutil.NewReader(r)
	flags := rr.ReadByte()
	if flags == 0x00 {
		return rr.Err
	}
	a.FungibleTokens = *NewEmptyFungibleTokens()
	if (flags & hasBaseTokens) != 0 {
		a.BaseTokens = iotago.BaseToken(rr.ReadAmount64())
	}
	if (flags & hasNativeTokens) != 0 {
		size := rr.ReadSize16()
		for i := 0; i < size; i++ {
			var id iotago.NativeTokenID
			rr.ReadN(id[:])
			a.NativeTokens[id] = rr.ReadUint256()
		}
	}
	if (flags & hasNFTs) != 0 {
		size := rr.ReadSize16()
		a.NFTs = make([]iotago.NFTID, size)
		for i := range a.NFTs {
			rr.ReadN(a.NFTs[i][:])
		}
	}
	return rr.Err
}

func (a *Assets) Write(w io.Writer) error {
	ww := rwutil.NewWriter(w)
	if a.IsEmpty() {
		ww.WriteByte(0x00)
		return ww.Err
	}

	var flags byte
	if a.BaseTokens != 0 {
		flags |= hasBaseTokens
	}
	if len(a.NativeTokens) != 0 {
		flags |= hasNativeTokens
	}
	if len(a.NFTs) != 0 {
		flags |= hasNFTs
	}

	ww.WriteByte(flags)
	if (flags & hasBaseTokens) != 0 {
		ww.WriteAmount64(uint64(a.BaseTokens))
	}
	if (flags & hasNativeTokens) != 0 {
		ww.WriteSize16(len(a.NativeTokens))
		for _, id := range a.NativeTokenIDsSorted() {
			ww.WriteN(id[:])
			ww.WriteUint256(a.NativeTokens[id])
		}
	}
	if (flags & hasNFTs) != 0 {
		ww.WriteSize16(len(a.NFTs))
		sort.Slice(a.NFTs, func(lhs, rhs int) bool {
			return bytes.Compare(a.NFTs[lhs][:], a.NFTs[rhs][:]) < 0
		})
		for _, nft := range a.NFTs {
			ww.WriteN(nft[:])
		}
	}
	return ww.Err
}
