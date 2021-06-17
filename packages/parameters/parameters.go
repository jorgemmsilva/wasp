package parameters

import (
	"github.com/iotaledger/wasp/plugins/config"
	flag "github.com/spf13/pflag"
)

const (
	LoggerLevel             = "logger.level"
	LoggerDisableCaller     = "logger.disableCaller"
	LoggerDisableStacktrace = "logger.disableStacktrace"
	LoggerEncoding          = "logger.encoding"
	LoggerOutputPaths       = "logger.outputPaths"
	LoggerDisableEvents     = "logger.disableEvents"

	DatabaseDir      = "database.directory"
	DatabaseInMemory = "database.inMemory"

	WebAPIBindAddress    = "webapi.bindAddress"
	WebAPIAdminWhitelist = "webapi.adminWhitelist"
	WebAPIAuth           = "webapi.auth"

	DashboardBindAddress       = "dashboard.bindAddress"
	DashboardExploreAddressURL = "dashboard.exploreAddressUrl"
	DashboardAuth              = "dashboard.auth"

	NodeAddress = "nodeconn.address"

	PeeringMyNetID   = "peering.netid"
	PeeringPort      = "peering.port"
	PeeringNeighbors = "peering.neighbors"

	NanomsgPublisherPort = "nanomsg.port"

	IpfsGatewayAddress = "ipfs.gatewayAddress"

	OffledgerGossipUpToNPeers = "offledger.gossipUpToNPeers"
)

func InitFlags() {
	flag.String(LoggerLevel, "info", "log level")
	flag.Bool(LoggerDisableCaller, false, "disable caller info in log")
	flag.Bool(LoggerDisableStacktrace, false, "disable stack trace in log")
	flag.String(LoggerEncoding, "console", "log encoding")
	flag.StringSlice(LoggerOutputPaths, []string{"stdout", "goshimmer.log"}, "log output paths")
	flag.Bool(LoggerDisableEvents, true, "disable logger events")

	flag.String(DatabaseDir, "waspdb", "path to the database folder")
	flag.Bool(DatabaseInMemory, false, "whether the database is only kept in memory and not persisted")

	flag.String(WebAPIBindAddress, "127.0.0.1:8080", "the bind address for the web API")
	flag.StringSlice(WebAPIAdminWhitelist, []string{}, "IP whitelist for /adm wndpoints")
	flag.StringToString(WebAPIAuth, nil, "authentication scheme for web API")

	flag.String(DashboardBindAddress, "127.0.0.1:7000", "the bind address for the node dashboard")
	flag.String(DashboardExploreAddressURL, "", "URL to add as href to addresses in the dashboard [default: <nodeconn.address>:8081/explorer/address]")
	flag.StringToString(DashboardAuth, nil, "authentication scheme for the node dashboard")

	flag.String(NodeAddress, "127.0.0.1:5000", "node host address")

	flag.Int(PeeringPort, 4000, "port for Wasp committee connection/peering")
	flag.String(PeeringMyNetID, "127.0.0.1:4000", "node host address as it is recognized by other peers")
	flag.StringSlice(PeeringNeighbors, []string{}, "list of neighbors: known peer netIDs")

	flag.Int(NanomsgPublisherPort, 5550, "the port for nanomsg even publisher")

	flag.String(IpfsGatewayAddress, "https://ipfs.io/", "the address of HTTP(s) gateway to which download from ipfs requests will be forwarded")

	flag.Int(OffledgerGossipUpToNPeers, 10, "number of peers an offledger request is gossiped to")
}

func GetBool(name string) bool {
	return config.Node.Bool(name)
}

func GetString(name string) string {
	return config.Node.String(name)
}

func GetStringSlice(name string) []string {
	return config.Node.Strings(name)
}

func GetInt(name string) int {
	return config.Node.Int(name)
}

func GetStringToString(name string) map[string]string {
	return config.Node.StringMap(name)
}
