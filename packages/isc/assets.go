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

type FungibleTokens struct {
	BaseTokens   iotago.BaseToken     `json:"baseTokens"`
	NativeTokens []*NativeTokenAmount `json:"nativeTokens"`
}

type NativeTokenAmount struct {
	ID     iotago.NativeTokenID `json:"id"`
	Amount *big.Int             `json:"amount"`
}

func (n *NativeTokenAmount) Clone() *NativeTokenAmount {
	return &NativeTokenAmount{
		ID:     n.ID,
		Amount: new(big.Int).Set(n.Amount),
	}
}

func NewFungibleTokens(baseTokens iotago.BaseToken, tokens []*NativeTokenAmount) *FungibleTokens {
	return &FungibleTokens{
		BaseTokens:   baseTokens,
		NativeTokens: tokens,
	}
}

func NewEmptyFungibleTokens() *FungibleTokens {
	return &FungibleTokens{}
}

type Assets struct {
	*FungibleTokens
	NFTs []iotago.NFTID `json:"nfts"`
}

var BaseTokenID = []byte{}

func NewAssets(baseTokens iotago.BaseToken, tokens []*NativeTokenAmount, nfts ...iotago.NFTID) *Assets {
	ret := &Assets{FungibleTokens: NewFungibleTokens(baseTokens, tokens)}
	if len(nfts) != 0 {
		ret.AddNFTs(nfts...)
	}
	return ret
}

func NewAssetsBaseTokens(amount iotago.BaseToken) *Assets {
	return &Assets{FungibleTokens: NewFungibleTokens(amount, nil)}
}

func NewEmptyAssets() *Assets {
	return &Assets{FungibleTokens: NewEmptyFungibleTokens()}
}

func AssetsFromBytes(b []byte) (*Assets, error) {
	if len(b) == 0 {
		return NewEmptyAssets(), nil
	}
	return rwutil.ReadFromBytes(b, NewEmptyAssets())
}

func FungibleTokensFromDict(d dict.Dict) (*FungibleTokens, error) {
	ret := NewEmptyFungibleTokens()
	for key, val := range d {
		if IsBaseToken([]byte(key)) {
			ret.BaseTokens = iotago.BaseToken(new(big.Int).SetBytes(d.Get(kv.Key(BaseTokenID))).Uint64())
			continue
		}
		id, err := NativeTokenIDFromBytes([]byte(key))
		if err != nil {
			return nil, fmt.Errorf("Assets: %w", err)
		}
		token := &NativeTokenAmount{
			ID:     id,
			Amount: new(big.Int).SetBytes(val),
		}
		ret.NativeTokens = append(ret.NativeTokens, token)
	}
	return ret, nil
}

func FungibleTokensFromNativeTokenSum(baseTokens iotago.BaseToken, tokens iotago.NativeTokenSum) *FungibleTokens {
	ret := NewEmptyFungibleTokens()
	ret.BaseTokens = baseTokens
	for id, val := range tokens {
		ret.NativeTokens = append(ret.NativeTokens, &NativeTokenAmount{
			ID:     id,
			Amount: val,
		})
	}
	return ret
}

func FungibleTokensFromOutput(o iotago.Output) *FungibleTokens {
	ret := &FungibleTokens{
		BaseTokens: o.BaseTokenAmount(),
	}
	if o.FeatureSet().HasNativeTokenFeature() {
		ret.NativeTokens = []*NativeTokenAmount{(*NativeTokenAmount)(o.FeatureSet().NativeToken())}
	}
	return ret
}

func AssetsFromOutput(o iotago.Output, oid iotago.OutputID) *Assets {
	ret := &Assets{FungibleTokens: FungibleTokensFromOutput(o)}
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

func MustAssetsFromBytes(b []byte) *Assets {
	ret, err := AssetsFromBytes(b)
	if err != nil {
		panic(err)
	}
	return ret
}

// returns nil if nil pointer receiver is cloned
func (a *FungibleTokens) ToAssets() *Assets {
	return NewAssets(a.BaseTokens, a.NativeTokens)
}

// returns nil if nil pointer receiver is cloned
func (a *FungibleTokens) Clone() *FungibleTokens {
	if a == nil {
		return nil
	}
	return NewFungibleTokens(
		a.BaseTokens,
		lo.Map(a.NativeTokens, func(item *NativeTokenAmount, index int) *NativeTokenAmount {
			return item.Clone()
		}),
	)
}

// returns nil if nil pointer receiver is cloned
func (a *Assets) Clone() *Assets {
	if a == nil {
		return nil
	}
	return &Assets{
		FungibleTokens: a.FungibleTokens.Clone(),
		NFTs:           slices.Clone(a.NFTs),
	}
}

func (a *Assets) AddNFTs(nfts ...iotago.NFTID) *Assets {
	nftMap := make(map[iotago.NFTID]bool)
	nfts = append(nfts, a.NFTs...)
	for _, nftid := range nfts {
		nftMap[nftid] = true
	}
	a.NFTs = make([]iotago.NFTID, len(nftMap))
	i := 0
	for nftid := range nftMap {
		a.NFTs[i] = nftid
		i++
	}
	return a
}

func (a *FungibleTokens) AmountNativeToken(nativeTokenID iotago.NativeTokenID) *big.Int {
	for _, t := range a.NativeTokens {
		if t.ID == nativeTokenID {
			return t.Amount
		}
	}
	return big.NewInt(0)
}

func (a *FungibleTokens) String() string {
	ret := fmt.Sprintf("base tokens: %d", a.BaseTokens)
	if len(a.NativeTokens) > 0 {
		ret += fmt.Sprintf(", tokens (%d):", len(a.NativeTokens))
	}
	for _, nt := range a.NativeTokens {
		ret += fmt.Sprintf("\n       %s: %s", nt.ID.String(), nt.Amount.Text(10))
	}
	return ret
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

func (a *FungibleTokens) Equals(b *FungibleTokens) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.BaseTokens != b.BaseTokens {
		return false
	}
	if !maps.EqualFunc(a.NativeTokenSum(), b.NativeTokenSum(), func(nt1, nt2 *big.Int) bool {
		return nt1.Cmp(nt2) == 0
	}) {
		return false
	}
	return true
}

func (a *Assets) Equals(b *Assets) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if !a.FungibleTokens.Equals(b.FungibleTokens) {
		return false
	}
	if !maps.Equal(a.NFTSet(), b.NFTSet()) {
		return false
	}
	return true
}

func (a *FungibleTokens) Geq(b *FungibleTokens) bool {
	if a.IsEmpty() {
		return b.IsEmpty()
	}
	if b.IsEmpty() {
		return true
	}
	if a.BaseTokens < b.BaseTokens {
		return false
	}

	aNTs := a.NativeTokenSum()
	for _, bNT := range b.NativeTokens {
		aNT, ok := aNTs[bNT.ID]
		if !ok || aNT.Cmp(bNT.Amount) < 0 {
			return false
		}
	}

	return true
}

func (a *Assets) Geq(b *Assets) bool {
	if a.IsEmpty() {
		return b.IsEmpty()
	}
	if b.IsEmpty() {
		return true
	}
	if !a.FungibleTokens.Geq(b.FungibleTokens) {
		return false
	}
	return lo.Every(b.NFTs, a.NFTs)
}

// Spend subtracts assets from the current set.
// Mutates receiver `a` !
// If budget is not enough, returns false and leaves receiver untouched
func (a *FungibleTokens) Spend(toSpend *FungibleTokens) bool {
	if !a.Geq(toSpend) {
		return false
	}
	if toSpend.IsEmpty() { // necessary if a == nil
		return true
	}

	a.BaseTokens -= toSpend.BaseTokens

	aNTs := a.NativeTokenSum()
	for id, amt := range toSpend.NativeTokenSum() {
		aNTs[id] = aNTs[id].Sub(aNTs[id], amt)
	}
	a.NativeTokens = nativeTokensFromSet(aNTs)

	return true
}

// Spend subtracts assets from the current set.
// Mutates receiver `a` !
// If budget is not enough, returns false and leaves receiver untouched
func (a *Assets) Spend(toSpend *Assets) bool {
	if !a.Geq(toSpend) {
		return false
	}
	if toSpend.IsEmpty() { // necessary if a == nil
		return true
	}

	a.FungibleTokens.Spend(toSpend.FungibleTokens)
	a.NFTs = lo.Reject(a.NFTs, func(id iotago.NFTID, i int) bool {
		return lo.Contains(toSpend.NFTs, id)
	})
	return true
}

func (a *FungibleTokens) NativeTokenSum() iotago.NativeTokenSum {
	ret := iotago.NativeTokenSum{}
	for _, nt := range a.NativeTokens {
		ret[nt.ID] = nt.Amount
	}
	return ret
}

func (a *Assets) NFTSet() map[iotago.NFTID]bool {
	ret := map[iotago.NFTID]bool{}
	for _, nft := range a.NFTs {
		ret[nft] = true
	}
	return ret
}

func (a *FungibleTokens) Add(b *FungibleTokens) *FungibleTokens {
	a.BaseTokens += b.BaseTokens
	resultTokens := a.NativeTokenSum()
	for _, nativeToken := range b.NativeTokens {
		if resultTokens[nativeToken.ID] == nil {
			resultTokens[nativeToken.ID] = new(big.Int)
		}
		resultTokens[nativeToken.ID].Add(
			resultTokens[nativeToken.ID],
			nativeToken.Amount,
		)
	}
	a.NativeTokens = nativeTokensFromSet(resultTokens)
	return a
}

func (a *Assets) Add(b *Assets) *Assets {
	a.FungibleTokens.Add(b.FungibleTokens)
	a.AddNFTs(b.NFTs...)
	return a
}

func (a *FungibleTokens) IsEmpty() bool {
	return a == nil || (a.BaseTokens == 0 && len(a.NativeTokens) == 0)
}

func (a *Assets) IsEmpty() bool {
	return a == nil || a.FungibleTokens.IsEmpty() && len(a.NFTs) == 0
}

func (a *FungibleTokens) AddBaseTokens(amount iotago.BaseToken) *FungibleTokens {
	a.BaseTokens += amount
	return a
}

func (a *Assets) AddBaseTokens(amount iotago.BaseToken) *Assets {
	a.FungibleTokens.AddBaseTokens(amount)
	return a
}

func (a *FungibleTokens) AddNativeTokens(nativeTokenID iotago.NativeTokenID, amount interface{}) *FungibleTokens {
	b := NewFungibleTokens(0, []*NativeTokenAmount{
		{
			ID:     nativeTokenID,
			Amount: util.ToBigInt(amount),
		},
	})
	return a.Add(b)
}

func (a *Assets) AddNativeTokens(nativeTokenID iotago.NativeTokenID, amount interface{}) *Assets {
	a.FungibleTokens.AddNativeTokens(nativeTokenID, amount)
	return a
}

func (a *FungibleTokens) ToDict() dict.Dict {
	ret := dict.New()
	ret.Set(kv.Key(BaseTokenID), new(big.Int).SetUint64(uint64(a.BaseTokens)).Bytes())
	for _, nativeToken := range a.NativeTokens {
		ret.Set(kv.Key(nativeToken.ID[:]), nativeToken.Amount.Bytes())
	}
	return ret
}

func (a *Assets) fillEmptyNFTIDs(output iotago.Output, outputID iotago.OutputID) *Assets {
	if a == nil {
		return nil
	}

	nftOutput, ok := output.(*iotago.NFTOutput)
	if !ok {
		return a
	}

	// see if there is an empty NFTID in the assets (this can happen if the NTF is minted as a request to the chain)
	for i, nftID := range a.NFTs {
		if nftID.Empty() {
			a.NFTs[i] = util.NFTIDFromNFTOutput(nftOutput, outputID)
		}
	}
	return a
}

func nativeTokensFromSet(set iotago.NativeTokenSum) []*NativeTokenAmount {
	ret := make([]*NativeTokenAmount, 0, len(set))
	for id, amt := range set {
		if amt.Sign() == 0 {
			continue
		}
		ret = append(ret, &NativeTokenAmount{
			ID:     id,
			Amount: amt,
		})
	}
	return ret
}

// IsBaseToken return whether a given tokenID represents the base token
func IsBaseToken(tokenID []byte) bool {
	return bytes.Equal(tokenID, BaseTokenID)
}

// Since we are encoding a nil assets pointer with a byte already,
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
	if (flags & hasBaseTokens) != 0 {
		a.BaseTokens = iotago.BaseToken(rr.ReadAmount64())
	}
	if (flags & hasNativeTokens) != 0 {
		size := rr.ReadSize16()
		a.NativeTokens = make([]*NativeTokenAmount, size)
		for i := range a.NativeTokens {
			nativeToken := new(NativeTokenAmount)
			a.NativeTokens[i] = nativeToken
			rr.ReadN(nativeToken.ID[:])
			nativeToken.Amount = rr.ReadUint256()
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
		sort.Slice(a.NativeTokens, func(lhs, rhs int) bool {
			return bytes.Compare(a.NativeTokens[lhs].ID[:], a.NativeTokens[rhs].ID[:]) < 0
		})
		for _, nativeToken := range a.NativeTokens {
			ww.WriteN(nativeToken.ID[:])
			ww.WriteUint256(nativeToken.Amount)
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

func (a *Assets) WithMana(m iotago.Mana) *AssetsWithMana {
	return NewAssetsWithMana(a, m)
}

type AssetsWithMana struct {
	*Assets
	Mana iotago.Mana
}

func NewAssetsWithMana(assets *Assets, mana iotago.Mana) *AssetsWithMana {
	return &AssetsWithMana{Assets: assets, Mana: mana}
}

func NewEmptyAssetsWithMana() *AssetsWithMana {
	return NewAssetsWithMana(NewEmptyAssets(), 0)
}

func (a *AssetsWithMana) Geq(b *AssetsWithMana) bool {
	if !a.Assets.Geq(b.Assets) {
		return false
	}
	return a.Mana > b.Mana
}

func (a *AssetsWithMana) Equals(b *AssetsWithMana) bool {
	return a.Assets.Equals(b.Assets) && a.Mana == b.Mana
}

func (a *AssetsWithMana) Add(b *AssetsWithMana) {
	a.Assets.Add(b.Assets)
	a.Mana += b.Mana
}
