package chains

import (
	"context"
	"time"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/app"
	"github.com/iotaledger/wasp/packages/chain"
	"github.com/iotaledger/wasp/packages/chains"
	"github.com/iotaledger/wasp/packages/database/dbmanager"
	"github.com/iotaledger/wasp/packages/metrics"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/vm/processors"
)

func init() {
	CoreComponent = &app.CoreComponent{
		Component: &app.Component{
			Name:           "Chains",
			DepsFunc:       func(cDeps dependencies) { deps = cDeps },
			Params:         params,
			InitConfigPars: initConfigPars,
			Provide:        provide,
			Run:            run,
		},
	}
}

var (
	CoreComponent *app.CoreComponent
	deps          dependencies
)

type dependencies struct {
	dig.In

	Chains          *chains.Chains
	Metrics         *metrics.Metrics `optional:"true"`
	DefaultRegistry registry.Registry
}

func initConfigPars(c *dig.Container) error {
	type cfgResult struct {
		dig.Out
		APICacheTTL time.Duration `name:"apiCacheTTL"`
	}

	if err := c.Provide(func() cfgResult {
		return cfgResult{
			APICacheTTL: ParamsChains.APICacheTTL,
		}
	}); err != nil {
		CoreComponent.LogPanic(err)
	}

	return nil
}

func provide(c *dig.Container) error {
	type chainsDeps struct {
		dig.In

		ProcessorsConfig       *processors.Config
		DatabaseManager        *dbmanager.DBManager
		DefaultNetworkProvider peering.NetworkProvider `name:"defaultNetworkProvider"`
		NodeConnection         chain.NodeConnection
	}

	type chainsResult struct {
		dig.Out

		Chains *chains.Chains
	}

	if err := c.Provide(func(chDeps chainsDeps) chainsResult {
		return chainsResult{
			Chains: chains.New(
				CoreComponent.Logger(),
				chDeps.NodeConnection,
				chDeps.ProcessorsConfig,
				ParamsChains.BroadcastUpToNPeers,
				ParamsChains.BroadcastInterval,
				ParamsChains.PullMissingRequestsFromCommittee,
				chDeps.DefaultNetworkProvider,
				chDeps.DatabaseManager.GetOrCreateKVStore,
				ParamsRawBlocks.Enabled,
				ParamsRawBlocks.Directory,
			),
		}
	}); err != nil {
		CoreComponent.LogPanic(err)
	}

	return nil
}

func run() error {
	err := CoreComponent.Daemon().BackgroundWorker(CoreComponent.Name, func(ctx context.Context) {
		if err := deps.Chains.Run(ctx, deps.DefaultRegistry, deps.Metrics); err != nil {
			CoreComponent.LogErrorf("failed to start chains: %v", err)
			return
		}

		<-ctx.Done()
	}, parameters.PriorityChains)
	if err != nil {
		CoreComponent.LogError(err)
		return err
	}

	return nil
}
