package transaction

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
)

func alternateSign(signMsg []byte, signer iotago.AddressSigner, addrKeys ...iotago.AddressKeys) ([]iotago.Signature, error) {
	sigs := make([]iotago.Signature, len(addrKeys))
	for i, v := range addrKeys {
		sig, err := signer.Sign(v.Address, signMsg)
		if err != nil {
			return nil, err
		}
		sigs[i] = sig
	}

	return sigs, nil
}

func SignTransaction(tx *iotago.Transaction, keyPair cryptolib.VariantKeyPair) ([]iotago.Signature, error) {
	signMsg, err := tx.SigningMessage()
	if err != nil {
		return nil, err
	}

	signer := keyPair.AsAddressSigner()
	addressKeys := keyPair.AddressKeysForEd25519Address(keyPair.Address())

	return alternateSign(signMsg, signer, addressKeys)
}
