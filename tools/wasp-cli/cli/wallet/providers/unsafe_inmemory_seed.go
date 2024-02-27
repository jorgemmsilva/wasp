package providers

import (
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/tools/wasp-cli/cli/cliclients"
	"github.com/iotaledger/wasp/tools/wasp-cli/cli/config"
	"github.com/iotaledger/wasp/tools/wasp-cli/cli/wallet/wallets"
	"github.com/iotaledger/wasp/tools/wasp-cli/log"
)

type UnsafeInMemoryTestingSeed struct {
	cryptolib.VariantKeyPair
	addressIndex int
}

func newUnsafeInMemoryTestingSeed(keyPair *cryptolib.KeyPair, addressIndex int) *UnsafeInMemoryTestingSeed {
	return &UnsafeInMemoryTestingSeed{
		VariantKeyPair: keyPair,
		addressIndex:   addressIndex,
	}
}

func (i *UnsafeInMemoryTestingSeed) AddressIndex() int {
	return i.addressIndex
}

func LoadUnsafeInMemoryTestingSeed(addressIndex int) wallets.Wallet {
	seed, err := hexutil.Decode(config.GetTestingSeed())
	log.Check(err)

	useLegacyDerivation := config.GetUseLegacyDerivation()
	hrp := cliclients.API().ProtocolParameters().Bech32HRP()
	keyPair := cryptolib.KeyPairFromSeed(cryptolib.SubSeed(seed, uint32(addressIndex), hrp, useLegacyDerivation))

	return newUnsafeInMemoryTestingSeed(keyPair, addressIndex)
}

func CreateUnsafeInMemoryTestingSeed() {
	seed := cryptolib.NewSeed()
	config.SetTestingSeed(hexutil.Encode(seed[:]))

	log.Printf("New testing seed saved inside the config file.\n")
}
