package wallet

import (
	"github.com/spf13/viper"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/hexutil"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/tools/wasp-cli/cli/cliclients"
	"github.com/iotaledger/wasp/tools/wasp-cli/log"
)

var AddressIndex int

type Wallet struct {
	KeyPair      *cryptolib.KeyPair
	AddressIndex int
}

func Load() *Wallet {
	seedHex := viper.GetString("wallet.seed")
	useLegacyDerivation := viper.GetBool("wallet.useLegacyDerivation")
	if seedHex == "" {
		log.Fatal("call `init` first")
	}

	masterSeed, err := hexutil.DecodeHex(seedHex)
	log.Check(err)

	kp := cryptolib.KeyPairFromSeed(cryptolib.SubSeed(masterSeed, uint32(AddressIndex), cliclients.API().ProtocolParameters().Bech32HRP(), useLegacyDerivation))

	return &Wallet{KeyPair: kp, AddressIndex: AddressIndex}
}

func (w *Wallet) PrivateKey() *cryptolib.PrivateKey {
	return w.KeyPair.GetPrivateKey()
}

func (w *Wallet) Address() iotago.Address {
	return w.KeyPair.GetPublicKey().AsEd25519Address()
}
