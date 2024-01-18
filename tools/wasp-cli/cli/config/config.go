package config

import (
	"fmt"

	"github.com/spf13/viper"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/tools/wasp-cli/log"
)

var (
	ConfigPath        string
	WaitForCompletion bool
)

func Read() {
	viper.SetConfigFile(ConfigPath)
	_ = viper.ReadInConfig()
}

func L1APIAddress() string {
	host := viper.GetString("l1.apiAddress")
	return host
}

func L1FaucetAddress() string {
	address := viper.GetString("l1.faucetAddress")
	return address
}

func GetToken(node string) string {
	return viper.GetString(fmt.Sprintf("authentication.wasp.%s.token", node))
}

func SetToken(node, token string) {
	Set(fmt.Sprintf("authentication.wasp.%s.token", node), token)
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
