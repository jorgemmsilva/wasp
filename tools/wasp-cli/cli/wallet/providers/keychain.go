package providers

import (
	"reflect"

	"github.com/spf13/viper"

	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/tools/wasp-cli/cli/cliclients"
	"github.com/iotaledger/wasp/tools/wasp-cli/cli/config"
	"github.com/iotaledger/wasp/tools/wasp-cli/cli/wallet/wallets"
	"github.com/iotaledger/wasp/tools/wasp-cli/log"
)

type KeyChainWallet struct {
	cryptolib.VariantKeyPair
	addressIndex int
}

func newInMemoryWallet(keyPair *cryptolib.KeyPair, addressIndex int) *KeyChainWallet {
	return &KeyChainWallet{
		VariantKeyPair: keyPair,
		addressIndex:   addressIndex,
	}
}

func (i *KeyChainWallet) AddressIndex() int {
	return i.addressIndex
}

func LoadKeyChain(addressIndex int) wallets.Wallet {
	seed, err := config.GetKeyChain().GetSeed()
	log.Check(err)

	hrp := cliclients.API().ProtocolParameters().Bech32HRP()
	useLegacyDerivation := config.GetUseLegacyDerivation()

	keyPair := cryptolib.KeyPairFromSeed(cryptolib.SubSeed(seed[:], uint32(addressIndex), hrp, useLegacyDerivation))

	return newInMemoryWallet(keyPair, addressIndex)
}

func CreateKeyChain(overwrite bool) {
	oldSeed, _ := config.GetKeyChain().GetSeed() //nolint: staticcheck, wastedassign

	if len(oldSeed) == cryptolib.SeedSize && !overwrite {
		log.Printf("You already have an existing seed inside your Keychain.\nCalling `init` will *replace* it with a new one.\n")
		log.Printf("Run `wasp-cli init --overwrite` to continue with the initialization.\n")
		log.Fatalf("The cli will now exit.")
	}

	seed := cryptolib.NewSeed()
	err := config.GetKeyChain().SetSeed(seed)
	log.Check(err)

	log.Printf("New seed stored inside the Keychain.\n")
}

func MigrateKeyChain(seed cryptolib.Seed) {
	err := config.GetKeyChain().SetSeed(seed)
	log.Check(err)
	log.Printf("Seed migrated to Keychain.\nProceeding with seed validation.\n")

	kcSeed, err := config.GetKeyChain().GetSeed()
	log.Check(err)

	if reflect.DeepEqual(kcSeed[:], seed[:]) {
		log.Printf("Seed has been successfully validated.\n")
		config.RemoveSeedForMigration()
		err = viper.WriteConfig()
		log.Check(err)
		log.Printf("Seed was removed from the config file\n")
	} else {
		log.Fatalf("Seed mismatch between Keychain and the config file.\nMigration failed.\n")
	}
}
