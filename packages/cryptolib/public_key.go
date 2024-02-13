package cryptolib

import (
	"bytes"
	"crypto/ed25519"
	"fmt"
	"io"

	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/group/edwards25519"

	hiveEd25519 "github.com/iotaledger/hive.go/crypto/ed25519"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/hexutil"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

// TODO we return a pointer (*PublicKey) everywhere, but we could just return (PubKey)

type PublicKey ed25519.PublicKey

type PublicKeyKey [ed25519.PublicKeySize]byte

func publicKeyFromCrypto(cryptoPublicKey ed25519.PublicKey) *PublicKey {
	ret := PublicKey(cryptoPublicKey)
	return &ret
}

func NewEmptyPublicKey() *PublicKey {
	return &PublicKey{}
}

func PublicKeyFromString(s string) (publicKey *PublicKey, err error) {
	bytes, err := hexutil.DecodeHex(s)
	if err != nil {
		return publicKey, fmt.Errorf("failed to parse public key %s from hex string: %w", s, err)
	}
	return PublicKeyFromBytes(bytes)
}

func PublicKeyFromBytes(publicKeyBytes []byte) (*PublicKey, error) {
	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("unexpected bytes length, expected: %d, got: %d", ed25519.PublicKeySize, len(publicKeyBytes))
	}

	ret := make([]byte, ed25519.PublicKeySize)
	copy(ret, publicKeyBytes)
	return (*PublicKey)(&ret), nil
}

func (pkT *PublicKey) Clone() *PublicKey {
	ret := make([]byte, ed25519.PublicKeySize)
	copy(ret, *pkT)
	return (*PublicKey)(&ret)
}

func (pkT *PublicKey) AsBytes() []byte {
	return (*pkT)[:]
}

func (pkT *PublicKey) AsKey() PublicKeyKey {
	ret := PublicKeyKey{}
	copy(ret[:], *pkT)
	return ret
}

func (pkT *PublicKey) AsEd25519PubKey() ed25519.PublicKey {
	return ed25519.PublicKey(*pkT)
}

func (pkT *PublicKey) AsHiveEd25519PubKey() hiveEd25519.PublicKey {
	ret := hiveEd25519.PublicKey{}
	if len(*pkT) != len(ret) {
		panic("unexpected public key size")
	}
	copy(ret[:], *pkT)
	return ret
}

func (pkT *PublicKey) AsEd25519Address() *iotago.Ed25519Address {
	return iotago.Ed25519AddressFromPubKey(pkT.AsEd25519PubKey())
}

func (pkT *PublicKey) AsKyberPoint() (kyber.Point, error) {
	return PointFromBytes(*pkT, new(edwards25519.Curve))
}

func (pkT *PublicKey) Equals(other *PublicKey) bool {
	return bytes.Equal(*pkT, *other)
}

func (pkT *PublicKey) Verify(message, sig []byte) bool {
	return ed25519.Verify(pkT.AsEd25519PubKey(), message, sig)
}

func (pkT *PublicKey) String() string {
	return hexutil.EncodeHex(*pkT)
}

func (pkT *PublicKey) Read(r io.Reader) error {
	rr := rwutil.NewReader(r)
	*pkT = make([]byte, ed25519.PublicKeySize)
	rr.ReadN(*pkT)
	return rr.Err
}

func (pkT *PublicKey) Write(w io.Writer) error {
	ww := rwutil.NewWriter(w)
	if len(*pkT) != ed25519.PublicKeySize {
		panic(fmt.Sprintf("unexpected public key size for write: expected %d, got %d", ed25519.PublicKeySize, len(*pkT)))
	}
	ww.WriteN(*pkT)
	return ww.Err
}
