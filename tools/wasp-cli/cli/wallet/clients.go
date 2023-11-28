package wallet

import (
	"github.com/iotaledger/wasp/clients/apiclient"
	"github.com/iotaledger/wasp/clients/chainclient"
	"github.com/iotaledger/wasp/clients/scclient"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/tools/wasp-cli/cli/cliclients"
)

func ChainClient(waspClient *apiclient.APIClient, chainID isc.ChainID) *chainclient.Client {
	return chainclient.New(
		cliclients.L1Client(),
		waspClient,
		chainID,
		Load().KeyPair,
	)
}

func SCClient(apiClient *apiclient.APIClient, chainID isc.ChainID, contractHname isc.Hname) *scclient.SCClient {
	return scclient.New(ChainClient(apiClient, chainID), contractHname)
}
