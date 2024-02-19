package webapi

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pangpanglabs/echoswagger/v2"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/app/configuration"
	"github.com/iotaledger/hive.go/app/shutdown"
	hivedb "github.com/iotaledger/hive.go/db"
	"github.com/iotaledger/hive.go/web/websockethub"
	"github.com/iotaledger/wasp/packages/chain"
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/chains"
	"github.com/iotaledger/wasp/packages/daemon"
	"github.com/iotaledger/wasp/packages/database"
	"github.com/iotaledger/wasp/packages/dkg"
	"github.com/iotaledger/wasp/packages/evm/jsonrpc"
	"github.com/iotaledger/wasp/packages/metrics"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/publisher"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/users"
	"github.com/iotaledger/wasp/packages/webapi"
	"github.com/iotaledger/wasp/packages/webapi/websocket"
)

func init() {
	Component = &app.Component{
		Name:             "WebAPI",
		DepsFunc:         func(cDeps dependencies) { deps = cDeps },
		Params:           params,
		InitConfigParams: initConfigParams,
		IsEnabled:        func(_ *dig.Container) bool { return ParamsWebAPI.Enabled },
		Provide:          provide,
		Run:              run,
	}
}

var (
	Component *app.Component
	deps      dependencies
)

type dependencies struct {
	dig.In

	EchoSwagger        echoswagger.ApiRoot `name:"webapiServer"`
	WebsocketHub       *websockethub.Hub   `name:"websocketHub"`
	NodeConnection     chain.NodeConnection
	WebsocketPublisher *websocket.Service `name:"websocketService"`
}

func initConfigParams(c *dig.Container) error {
	type cfgResult struct {
		dig.Out
		WebAPIBindAddress string `name:"webAPIBindAddress"`
	}

	if err := c.Provide(func() cfgResult {
		return cfgResult{
			WebAPIBindAddress: ParamsWebAPI.BindAddress,
		}
	}); err != nil {
		Component.LogPanic(err.Error())
	}

	return nil
}

func provide(c *dig.Container) error {
	type webapiServerDeps struct {
		dig.In

		AppInfo                     *app.Info
		AppConfig                   *configuration.Configuration `name:"appConfig"`
		ShutdownHandler             *shutdown.ShutdownHandler
		APICacheTTL                 time.Duration `name:"apiCacheTTL"`
		Chains                      *chains.Chains
		ChainMetricsProvider        *metrics.ChainMetricsProvider
		ChainRecordRegistryProvider registry.ChainRecordRegistryProvider
		DKShareRegistryProvider     registry.DKShareRegistryProvider
		NodeIdentityProvider        registry.NodeIdentityProvider
		NetworkProvider             peering.NetworkProvider `name:"networkProvider"`
		NodeConnection              chain.NodeConnection
		TrustedNetworkManager       peering.TrustedNetworkManager `name:"trustedNetworkManager"`
		Node                        *dkg.Node
		UserManager                 *users.UserManager
		Publisher                   *publisher.Publisher
	}

	type webapiServerResult struct {
		dig.Out

		Echo               *echo.Echo          `name:"webapiEcho"`
		EchoSwagger        echoswagger.ApiRoot `name:"webapiServer"`
		WebsocketHub       *websockethub.Hub   `name:"websocketHub"`
		WebsocketPublisher *websocket.Service  `name:"websocketService"`
	}

	if err := c.Provide(func(deps webapiServerDeps) webapiServerResult {
		logger := Component.NewChildLogger("WebAPI/v2")

		echoSwagger := webapi.NewEcho(
			ParamsWebAPI.DebugRequestLoggerEnabled,
			&ParamsWebAPI.Limits,
			ParamsWebAPI.Auth.JWTConfig.Duration,
			deps.ChainMetricsProvider,
			deps.AppInfo.Version,
			Component,
		)

		hub, websocketService := webapi.InitWebsocket(
			Component,
			deps.Publisher,
			deps.NodeConnection.L1APIProvider().LatestAPI(),
			ParamsWebAPI.Limits.MaxTopicSubscriptionsPerClient,
		)

		evmIndexDBProvider := func() (*database.Database, error) {
			return database.DatabaseWithDefaultSettings(ParamsWebAPI.IndexDbPath, true, hivedb.EngineRocksDB, false)
		}

		webapi.Init(
			logger,
			echoSwagger,
			deps.AppInfo.Version,
			deps.AppConfig,
			deps.NetworkProvider,
			deps.TrustedNetworkManager,
			deps.UserManager,
			deps.ChainRecordRegistryProvider,
			deps.DKShareRegistryProvider,
			deps.NodeIdentityProvider,
			func() chaintypes.Chains {
				return deps.Chains
			},
			func() *dkg.Node {
				return deps.Node
			},
			deps.ShutdownHandler,
			deps.ChainMetricsProvider,
			ParamsWebAPI.Auth,
			deps.APICacheTTL,
			websocketService,
			evmIndexDBProvider,
			deps.Publisher,
			jsonrpc.NewParameters(
				ParamsWebAPI.Limits.Jsonrpc.MaxBlocksInLogsFilterRange,
				ParamsWebAPI.Limits.Jsonrpc.MaxLogsInResult,
				ParamsWebAPI.Limits.Jsonrpc.WebsocketRateLimitMessagesPerSecond,
				ParamsWebAPI.Limits.Jsonrpc.WebsocketRateLimitBurst,
				ParamsWebAPI.Limits.Jsonrpc.WebsocketConnectionCleanupDuration,
				ParamsWebAPI.Limits.Jsonrpc.WebsocketClientBlockDuration,
			),
			deps.NodeConnection.L1APIProvider().LatestAPI(),
			deps.NodeConnection.BaseTokenInfo(),
		)

		return webapiServerResult{
			EchoSwagger:        echoSwagger,
			WebsocketHub:       hub,
			WebsocketPublisher: websocketService,
		}
	}); err != nil {
		Component.LogPanic(err.Error())
	}

	return nil
}

func run() error {
	Component.LogInfof("Starting %s server ...", Component.Name)
	if err := Component.Daemon().BackgroundWorker(Component.Name, func(ctx context.Context) {
		Component.LogInfof("Starting %s server ...", Component.Name)
		if err := deps.NodeConnection.WaitUntilInitiallySynced(ctx); err != nil {
			Component.LogErrorf("failed to start %s, waiting for L1 node to become sync failed, error: %s", err.Error())
			return
		}

		Component.LogInfof("Starting %s server ... done", Component.Name)

		go func() {
			deps.EchoSwagger.Echo().Server.BaseContext = func(_ net.Listener) context.Context {
				// set BaseContext to be the same as the plugin, so that requests being processed don't hang the shutdown procedure
				return ctx
			}

			Component.LogInfof("You can now access the WebAPI using: http://%s", ParamsWebAPI.BindAddress)
			if err := deps.EchoSwagger.Echo().Start(ParamsWebAPI.BindAddress); err != nil && !errors.Is(err, http.ErrServerClosed) {
				Component.LogWarnf("Stopped %s server due to an error (%s)", Component.Name, err)
			}
		}()

		<-ctx.Done()

		Component.LogInfof("Stopping %s server ...", Component.Name)

		shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCtxCancel()

		//nolint:contextcheck // false positive
		if err := deps.EchoSwagger.Echo().Shutdown(shutdownCtx); err != nil {
			Component.LogWarn(err.Error())
		}

		Component.LogInfof("Stopping %s server ... done", Component.Name)
	}, daemon.PriorityWebAPI); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	if err := Component.Daemon().BackgroundWorker("WebAPI[WS]", func(ctx context.Context) {
		unhook := deps.WebsocketPublisher.EventHandler().AttachToEvents()
		defer unhook()

		deps.WebsocketHub.Run(ctx)
		Component.LogInfo("Stopping WebAPI[WS]")
	}, daemon.PriorityWebAPI); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}
