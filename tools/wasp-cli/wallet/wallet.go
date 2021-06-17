package wallet

import (
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/wasp/tools/wasp-cli/config"
	"github.com/iotaledger/wasp/tools/wasp-cli/log"
	"github.com/mr-tron/base58"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type WalletConfig struct {
	Seed []byte
}

type Wallet struct {
	seed *seed.Seed
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new wallet",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		seed := base58.Encode(seed.NewSeed().Bytes())
		viper.Set("wallet.seed", seed)
		log.Check(viper.WriteConfig())

		log.Printf("Initialized wallet seed in %s\n", config.ConfigPath)
		log.Printf("\nIMPORTANT: wasp-cli is alpha phase. The seed is currently being stored " +
			"in a plain text file which is NOT secure. Do not use this seed to store funds " +
			"in the mainnet!\n")
		log.Verbosef("\nSeed: %s\n", seed)
	},
}

func Load() *Wallet {
	seedb58 := viper.GetString("wallet.seed")
	if seedb58 == "" {
		log.Fatalf("call `init` first")
	}
	seedBytes, err := base58.Decode(seedb58)
	log.Check(err)
	return &Wallet{seed.NewSeed(seedBytes)}
}

var addressIndex int

func (w *Wallet) KeyPair() *ed25519.KeyPair {
	return w.seed.KeyPair(uint64(addressIndex))
}

func (w *Wallet) Address() ledgerstate.Address {
	return w.seed.Address(uint64(addressIndex)).Address()
}
