package cliclients

import (
	"context"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/clients/apiclient"
	"github.com/iotaledger/wasp/clients/apiextensions"
	"github.com/iotaledger/wasp/components/app"
	"github.com/iotaledger/wasp/packages/l1connection"
	"github.com/iotaledger/wasp/tools/wasp-cli/cli/config"
	"github.com/iotaledger/wasp/tools/wasp-cli/log"
)

var SkipCheckVersions bool

func WaspClientForHostName(name string) *apiclient.APIClient {
	apiAddress := config.MustWaspAPIURL(name)
	log.Verbosef("using Wasp host %s\n", apiAddress)

	client, err := apiextensions.WaspAPIClientByHostName(apiAddress)
	log.Check(err)

	client.GetConfig().Debug = log.DebugFlag
	client.GetConfig().AddDefaultHeader("Authorization", "Bearer "+config.GetToken(name))

	return client
}

func WaspClient(name string) *apiclient.APIClient {
	client := WaspClientForHostName(name)
	assertMatchingNodeVersion(name, client)
	return client
}

func VersionsMatch(v1, v2 string) bool {
	if v1 == "" || v2 == "" {
		return false
	}
	if v1 == v2 {
		return true
	}
	// sometimes one of the versions comes without the initial "v"
	if v1[0] == 'v' && v1[1:] == v2 {
		return true
	}
	if v2[0] == 'v' && v1 == v2[1:] {
		return true
	}
	return false
}

func assertMatchingNodeVersion(name string, client *apiclient.APIClient) {
	if SkipCheckVersions {
		return
	}
	nodeVersion, _, err := client.NodeApi.
		GetVersion(context.Background()).
		Execute()
	log.Check(err)
	if !VersionsMatch(app.Version, nodeVersion.Version) {
		log.Fatalf("node [%s] version: %s, does not match wasp-cli version: %s. You can skip this check by re-running with command with --skip-version-check",
			name, nodeVersion.Version, app.Version)
	}
}

var l1client l1connection.Client

func L1Client() l1connection.Client {
	log.Verbosef("using L1 API %s\n", config.L1APIAddress())
	if l1client == nil {
		l1client = l1connection.NewClient(
			l1connection.Config{
				APIAddress:    config.L1APIAddress(),
				FaucetAddress: config.L1FaucetAddress(),
			},
			log.HiveLogger(),
		)
	}
	return l1client
}

func API() iotago.API {
	return L1Client().APIProvider().LatestAPI()
}
