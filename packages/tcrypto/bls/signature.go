package bls

import (
	"bytes"
	"io"

	"github.com/mr-tron/base58"

	"github.com/iotaledger/hive.go/ierrors"
	"github.com/iotaledger/hive.go/serializer/v2/byteutils"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

// region Signature ////////////////////////////////////////////////////////////////////////////////////////////////////

// Signature is the type of a raw BLS signature.
type Signature [SignatureSize]byte

// SignatureFromBytes unmarshals a Signature from a sequence of bytes.
func SignatureFromBytes(b []byte) (signature Signature, err error) {
	if len(b) != SignatureSize {
		return Signature{}, ierrors.New("SignatureFromBytes: invalid bytes length")
	}
	copy(signature[:], b)
	return
}

// SignatureFromBase58EncodedString creates a Signature from a base58 encoded string.
func SignatureFromBase58EncodedString(base58EncodedString string) (signature Signature, err error) {
	bytes, err := base58.Decode(base58EncodedString)
	if err != nil {
		err = ierrors.Wrapf(ErrBase58DecodeFailed, "error while decoding base58 encoded Signature: %w", err)

		return
	}

	if signature, err = SignatureFromBytes(bytes); err != nil {
		err = ierrors.Wrap(err, "failed to parse Signature from bytes")

		return
	}

	return
}

func SignatureFromReader(r io.Reader) (signature Signature, err error) {
	err = rwutil.ReadN(r, signature[:])
	return
}

// Bytes returns a marshaled version of the Signature.
func (s Signature) Bytes() []byte {
	return s[:]
}

// Base58 returns a base58 encoded version of the Signature.
func (s Signature) Base58() string {
	return base58.Encode(s.Bytes())
}

// String returns a human-readable version of the signature.
func (s Signature) String() string {
	return s.Base58()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SignatureWithPublicKey ///////////////////////////////////////////////////////////////////////////////////////

// SignatureWithPublicKey is a combination of a PublicKey and a Signature that is required to perform operations like
// Signature- and PublicKey-aggregations.
type SignatureWithPublicKey struct {
	PublicKey PublicKey
	Signature Signature
}

// NewSignatureWithPublicKey is the constructor for SignatureWithPublicKey objects.
func NewSignatureWithPublicKey(publicKey PublicKey, signature Signature) SignatureWithPublicKey {
	return SignatureWithPublicKey{
		PublicKey: publicKey,
		Signature: signature,
	}
}

// SignatureWithPublicKeyFromBytes unmarshals a SignatureWithPublicKey from a sequence of bytes.
func SignatureWithPublicKeyFromBytes(b []byte) (signatureWithPublicKey SignatureWithPublicKey, err error) {
	buf := bytes.NewBuffer(b)
	r, err := SignatureWithPublicKeyFromReader(buf)
	if err == nil && buf.Available() != 0 {
		return SignatureWithPublicKey{}, ierrors.New("SignatureWithPublicKeyFromBytes: excess bytes")
	}
	return r, err
}

// SignatureWithPublicKeyFromBase58EncodedString creates a SignatureWithPublicKey from a base58 encoded string.
func SignatureWithPublicKeyFromBase58EncodedString(base58EncodedString string) (signatureWithPublicKey SignatureWithPublicKey, err error) {
	bytes, err := base58.Decode(base58EncodedString)
	if err != nil {
		err = ierrors.Wrapf(ErrBase58DecodeFailed, "error while decoding base58 encoded SignatureWithPublicKey: %w", err)

		return
	}

	if signatureWithPublicKey, err = SignatureWithPublicKeyFromBytes(bytes); err != nil {
		err = ierrors.Wrap(err, "failed to parse SignatureWithPublicKey from bytes")

		return
	}

	return
}

func SignatureWithPublicKeyFromReader(r io.Reader) (signatureWithPublicKey SignatureWithPublicKey, err error) {
	if signatureWithPublicKey.PublicKey, err = PublicKeyFromReader(r); err != nil {
		err = ierrors.Wrap(err, "failed to parse PublicKey from Reader")
		return
	}
	if signatureWithPublicKey.Signature, err = SignatureFromReader(r); err != nil {
		err = ierrors.Wrap(err, "failed to parse Signature from Reader")
		return
	}
	return
}

// IsValid returns true if the signature is correct for the given data.
func (s SignatureWithPublicKey) IsValid(data []byte) bool {
	return s.PublicKey.SignatureValid(data, s.Signature)
}

// Bytes returns the signature in bytes.
func (s SignatureWithPublicKey) Bytes() []byte {
	return byteutils.ConcatBytes(s.PublicKey.Bytes(), s.Signature.Bytes())
}

// Encode returns the signature in bytes.
func (s SignatureWithPublicKey) Encode() ([]byte, error) {
	return s.Bytes(), nil
}

// Encode returns the signature in bytes.
func (s *SignatureWithPublicKey) Decode(b []byte) error {
	decoded, err := SignatureWithPublicKeyFromBytes(b)
	if err != nil {
		return err
	}
	s.PublicKey = decoded.PublicKey
	s.Signature = decoded.Signature
	return nil
}

// Base58 returns a base58 encoded version of the SignatureWithPublicKey.
func (s SignatureWithPublicKey) Base58() string {
	return base58.Encode(s.Bytes())
}

// String returns a human-readable version of the SignatureWithPublicKey (base58 encoded).
func (s SignatureWithPublicKey) String() string {
	return base58.Encode(s.Bytes())
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
