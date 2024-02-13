package config

import (
	"fmt"

	"github.com/spf13/viper"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/tools/wasp-cli/log"
)

var (
	BaseDir           string
	ConfigPath        string
	WaitForCompletion bool
)

func Read() {
	locateBaseDir()
	locateConfigFile()

	viper.SetConfigFile(ConfigPath)
	_ = viper.ReadInConfig()
}

func L1APIAddress() string {
	host := viper.GetString("l1.apiaddress")
	if host == "" {
		log.Fatalf("l1.apiaddress not defined")
	}
	return host
}

func L1FaucetAddress() string {
	address := viper.GetString("l1.faucetaddress")
	if address == "" {
		log.Fatalf("l1.faucetaddress not defined")
	}
	return address
}

var keyChain keychain.KeyChain

func GetKeyChain() keychain.KeyChain {
	if keyChain == nil {
		if keychain.IsKeyChainAvailable() {
			keyChain = keychain.NewKeyChainZalando()
		} else {
			keyChain = keychain.NewKeyChainFile(BaseDir, cli.ReadPasswordFromStdin)
		}
	}

	return keyChain
}

func GetToken(node string) string {
	token, err := GetKeyChain().GetJWTAuthToken(node)
	log.Check(err)
	return token
}

func SetToken(node, token string) {
	err := GetKeyChain().SetJWTAuthToken(node, token)
	log.Check(err)
}

func MustWaspAPIURL(nodeName string) string {
	apiAddress := WaspAPIURL(nodeName)
	if apiAddress == "" {
		log.Fatalf("wasp webapi not defined for node: %s", nodeName)
	}
	return apiAddress
}

func WaspAPIURL(nodeName string) string {
	return viper.GetString(fmt.Sprintf("wasp.%s", nodeName))
}

func NodeAPIURLs(nodeNames []string) []string {
	hosts := make([]string, 0)
	for _, nodeName := range nodeNames {
		hosts = append(hosts, MustWaspAPIURL(nodeName))
	}
	return hosts
}

func Set(key string, value interface{}) {
	viper.Set(key, value)
	log.Check(viper.WriteConfig())
}

func AddWaspNode(name, apiURL string) {
	Set("wasp."+name, apiURL)
}

func AddChain(name, chainID string) {
	Set("chains."+name, chainID)
}

func GetChain(name string, networkPrefix iotago.NetworkPrefix) isc.ChainID {
	configChainID := viper.GetString("chains." + name)
	if configChainID == "" {
		log.Fatal(fmt.Sprintf("chain '%s' doesn't exist in config file", name))
	}
	chainID, err := isc.ChainIDFromBech32(configChainID, networkPrefix)
	log.Check(err)
	return chainID
}

func GetUseLegacyDerivation() bool {
	return viper.GetBool("wallet.useLegacyDerivation")
}

func GetWalletProviderString() string {
	return viper.GetString("wallet.provider")
}

func SetWalletProviderString(provider string) {
	Set("wallet.provider", provider)
}

// GetSeedForMigration is used to migrate the seed of the config file to a certain wallet provider.
func GetSeedForMigration() string {
	return viper.GetString("wallet.seed")
}

func GetWalletLogLevel() types.ILoggerConfigLevelFilter {
	logLevel := viper.GetString("wallet.loglevel")
	if logLevel == "" {
		return types.LevelFilterOff
	}

	return types.ILoggerConfigLevelFilter(logLevel)
}

func SetWalletLogLevel(filter types.ILoggerConfigLevelFilter) {
	viper.Set("wallet.loglevel", string(filter))
}
