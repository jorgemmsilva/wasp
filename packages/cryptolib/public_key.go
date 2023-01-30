package cryptolib

import (
	"crypto/ed25519"
	"errors"
	"fmt"

	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/group/edwards25519"

	iotago "github.com/iotaledger/iota.go/v3"
)

type PublicKey struct {
	Key ed25519.PublicKey
}

type PublicKeyKey [PublicKeySize]byte

const PublicKeySize = ed25519.PublicKeySize

func newPublicKeyFromCrypto(cryptoPublicKey ed25519.PublicKey) *PublicKey {
	return &PublicKey{cryptoPublicKey}
}

func NewEmptyPublicKey() *PublicKey {
	return &PublicKey{
		Key: make([]byte, PublicKeySize),
	}
}

func NewPublicKeyFromString(s string) (publicKey *PublicKey, err error) {
	bytes, err := iotago.DecodeHex(s)
	if err != nil {
		return publicKey, fmt.Errorf("failed to parse public key %s from hex string: %w", s, err)
	}
	return NewPublicKeyFromBytes(bytes)
}

func NewPublicKeyFromBytes(publicKeyBytes []byte) (*PublicKey, error) {
	if len(publicKeyBytes) < PublicKeySize {
		return nil, errors.New("bytes too short")
	}
	return &PublicKey{publicKeyBytes}, nil
}

func (pkT *PublicKey) Clone() *PublicKey {
	key := make([]byte, len(pkT.Key))
	copy(key, pkT.Key)
	return &PublicKey{Key: key}
}

func (pkT *PublicKey) AsBytes() []byte {
	return pkT.Key
}

func (pkT *PublicKey) AsKey() PublicKeyKey {
	var result [PublicKeySize]byte
	copy(result[:], pkT.Key)
	return result
}

func (pkT *PublicKey) AsEd25519Address() *iotago.Ed25519Address {
	ret := iotago.Ed25519AddressFromPubKey(pkT.Key)
	return &ret
}

func (pkT *PublicKey) AsKyberPoint() (kyber.Point, error) {
	group := new(edwards25519.Curve)
	point := group.Point()
	if err := point.UnmarshalBinary(pkT.AsBytes()); err != nil {
		return nil, err
	}
	return point, nil
}

func (pkT *PublicKey) Equals(other *PublicKey) bool {
	if len(pkT.Key) != len(other.Key) {
		return false
	}
	for i := range pkT.Key {
		if pkT.Key[i] != other.Key[i] {
			return false
		}
	}
	return true
}

func (pkT *PublicKey) Verify(message, sig []byte) bool {
	return ed25519.Verify(pkT.Key, message, sig)
}

func (pkT *PublicKey) String() string {
	return iotago.EncodeHex(pkT.Key)
}
