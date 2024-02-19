package wallets

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
)

type ExternalAddressSigner struct {
	keyPair cryptolib.VariantKeyPair
}

// EmptySignatureForAddress implements iotago.AddressSigner.
func (r *ExternalAddressSigner) EmptySignatureForAddress(addr iotago.Address) (signature iotago.Signature, err error) {
	panic("unimplemented")
}

// SignerUIDForAddress implements iotago.AddressSigner.
func (r *ExternalAddressSigner) SignerUIDForAddress(addr iotago.Address) (iotago.Identifier, error) {
	panic("unimplemented")
}

func (r *ExternalAddressSigner) Sign(addr iotago.Address, msg []byte) (signature iotago.Signature, err error) {
	return r.keyPair.Sign(addr, msg)
}
