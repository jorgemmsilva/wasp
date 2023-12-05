package cliclients

import (
	"context"

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

func assertMatchingNodeVersion(name string, client *apiclient.APIClient) {
	if SkipCheckVersions {
		return
	}
	nodeVersion, _, err := client.NodeApi.
		GetVersion(context.Background()).
		Execute()
	log.Check(err)
	if app.Version != "v"+nodeVersion.Version {
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
