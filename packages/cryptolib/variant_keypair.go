package cryptolib

import (
	iotago "github.com/iotaledger/iota.go/v4"
)

// VariantKeyPair originates from cryptolib.KeyPair
type VariantKeyPair interface {
	GetPublicKey() *PublicKey
	Address() *iotago.Ed25519Address
	AsAddressSigner() iotago.AddressSigner
	AddressKeysForEd25519Address(addr *iotago.Ed25519Address) iotago.AddressKeys
	SignBytes(data []byte) []byte
	iotago.AddressSigner
}
