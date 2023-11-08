package bls

import (
	"bytes"
	"io"

	"github.com/mr-tron/base58"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/sign/bdn"

	"github.com/iotaledger/hive.go/ierrors"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

// PublicKey is the type of BLS public keys.
type PublicKey struct {
	Point kyber.Point
}

// PublicKeyFromBytes creates a PublicKey from the given bytes.
func PublicKeyFromBytes(b []byte) (publicKey PublicKey, err error) {
	buf := bytes.NewBuffer(b)
	r, err := PublicKeyFromReader(buf)
	if err == nil && buf.Available() != 0 {
		return PublicKey{}, ierrors.New("PublicKeyFromBytes: excess bytes")
	}
	return r, err
}

// PublicKeyFromBase58EncodedString creates a PublicKey from a base58 encoded string.
func PublicKeyFromBase58EncodedString(base58String string) (publicKey PublicKey, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = ierrors.Wrapf(ErrBase58DecodeFailed, "error while decoding base58 encoded PublicKey: %w", err)

		return
	}

	if publicKey, err = PublicKeyFromBytes(bytes); err != nil {
		err = ierrors.Wrap(err, "failed to parse PublicKey from bytes")

		return
	}

	return
}

func PublicKeyFromReader(r io.Reader) (publicKey PublicKey, err error) {
	rr := rwutil.NewReader(r)
	var b [PublicKeySize]byte
	rr.ReadN(b[:])
	if rr.Err != nil {
		err = ierrors.Wrapf(ErrParseBytesFailed, "failed to read PublicKey bytes: %w", err)
		return
	}
	if err = publicKey.Point.UnmarshalBinary(b[:]); err != nil {
		err = ierrors.Wrapf(ErrParseBytesFailed, "failed to unmarshal PublicKey: %w", err)
		return
	}
	return
}

// SignatureValid reports whether the signature is valid for the given data.
func (p PublicKey) SignatureValid(data []byte, signature Signature) bool {
	return bdn.Verify(blsSuite, p.Point, data, signature.Bytes()) == nil
}

// Bytes returns a marshaled version of the PublicKey.
func (p PublicKey) Bytes() []byte {
	bytes, err := p.Point.MarshalBinary()
	if err != nil {
		panic(err)
	}

	return bytes
}

// Base58 returns a base58 encoded version of the PublicKey.
func (p PublicKey) Base58() string {
	return base58.Encode(p.Bytes())
}

// String returns a human-readable version of the PublicKey (base58 encoded).
func (p PublicKey) String() string {
	return base58.Encode(p.Bytes())
}
