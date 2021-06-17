package config

import (
	"fmt"
	"os"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/wasp/client"
	"github.com/iotaledger/wasp/client/goshimmer"
	"github.com/iotaledger/wasp/tools/wasp-cli/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	ConfigPath        string
	WaitForCompletion bool
)

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2), //nolint:gomnd
	Run: func(cmd *cobra.Command, args []string) {
		v := args[1]
		switch v {
		case "true":
			Set(args[0], true)
		case "false":
			Set(args[0], true)
		default:
			Set(args[0], v)
		}
	},
}

const (
	HostKindAPI     = "api"
	HostKindPeering = "peering"
	HostKindNanomsg = "nanomsg"
)

func Init(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().StringVarP(&ConfigPath, "config", "c", "wasp-cli.json", "path to wasp-cli.json")
	rootCmd.PersistentFlags().BoolVarP(&WaitForCompletion, "wait", "w", true, "wait for request completion")

	rootCmd.AddCommand(configSetCmd)
}

func Read() {
	viper.SetConfigFile(ConfigPath)
	_ = viper.ReadInConfig()
}

func GoshimmerAPIConfigVar() string {
	return "goshimmer." + HostKindAPI
}

func GoshimmerAPI() string {
	r := viper.GetString(GoshimmerAPIConfigVar())
	if r != "" {
		return r
	}
	return "127.0.0.1:8080"
}

func GoshimmerFaucetPoWTarget() int {
	key := "goshimmer.faucetPoWTarget"
	if !viper.IsSet(key) {
		return 0 // by default assume utxodb is on the other side
	}
	return viper.GetInt(key)
}

func GoshimmerClient() *goshimmer.Client {
	log.Verbosef("using Goshimmer host %s, faucet pow target %d\n", GoshimmerAPI(), GoshimmerFaucetPoWTarget())
	return goshimmer.NewClient(GoshimmerAPI(), GoshimmerFaucetPoWTarget())
}

func WaspClient() *client.WaspClient {
	// TODO: add authentication for /adm
	log.Verbosef("using Wasp host %s\n", WaspAPI())
	return client.NewWaspClient(WaspAPI())
}

func WaspAPI() string {
	r := viper.GetString("wasp." + HostKindAPI)
	if r != "" {
		return r
	}
	return committeeHost(HostKindAPI, 0)
}

func WaspNanomsg() string {
	r := viper.GetString("wasp." + HostKindNanomsg)
	if r != "" {
		return r
	}
	return committeeHost(HostKindNanomsg, 0)
}

func FindNodeBy(kind, v string) int {
	for i := 0; i < 100; i++ {
		if committeeHost(kind, i) == v {
			return i
		}
	}
	log.Fatalf("Cannot find node with %q = %q in configuration", kind, v)
	return 0
}

func CommitteeAPI(indices []int) []string {
	return committee(HostKindAPI, indices)
}

func CommitteePeering(indices []int) []string {
	return committee(HostKindPeering, indices)
}

func CommitteeNanomsg(indices []int) []string {
	return committee(HostKindNanomsg, indices)
}

func committee(kind string, indices []int) []string {
	hosts := make([]string, 0)
	for _, i := range indices {
		hosts = append(hosts, committeeHost(kind, i))
	}
	return hosts
}

func committeeConfigVar(kind string, i int) string {
	return fmt.Sprintf("wasp.%d.%s", i, kind)
}

func CommitteeAPIConfigVar(i int) string {
	return committeeConfigVar(HostKindAPI, i)
}

func CommitteePeeringConfigVar(i int) string {
	return committeeConfigVar(HostKindPeering, i)
}

func CommitteeNanomsgConfigVar(i int) string {
	return committeeConfigVar(HostKindNanomsg, i)
}

func committeeHost(kind string, i int) string {
	r := viper.GetString(committeeConfigVar(kind, i))
	if r != "" {
		return r
	}
	defaultPort := defaultWaspPort(kind, i)
	return fmt.Sprintf("127.0.0.1:%d", defaultPort)
}

//nolint:gomnd
func defaultWaspPort(kind string, i int) int {
	switch kind {
	case HostKindNanomsg:
		return 5550 + i
	case HostKindPeering:
		return 4000 + i
	case HostKindAPI:
		return 9090 + i
	}
	panic(fmt.Sprintf("no handler for kind %s", kind))
}

func Set(key string, value interface{}) {
	viper.Set(key, value)
	log.Check(viper.WriteConfig())
}

func TrySCAddress(scAlias string) ledgerstate.Address {
	b58 := viper.GetString("sc." + scAlias + ".address")
	if b58 == "" {
		return nil
	}
	address, err := ledgerstate.AddressFromBase58EncodedString(b58)
	log.Check(err)
	return address
}

func GetSCAddress(scAlias string) ledgerstate.Address {
	address := TrySCAddress(scAlias)
	if address == nil {
		log.Fatalf("call `%s set sc.%s.address` or deploy a contract first", os.Args[0], scAlias)
	}
	return address
}
