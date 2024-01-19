// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pangpanglabs/echoswagger/v2"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	websocketserver "nhooyr.io/websocket"

	hivedb "github.com/iotaledger/hive.go/kvstore/database"
	hivelog "github.com/iotaledger/hive.go/log"
	"github.com/iotaledger/hive.go/web/websockethub"
	"github.com/iotaledger/inx-app/pkg/httpserver"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/hexutil"
	"github.com/iotaledger/wasp/components/app"
	"github.com/iotaledger/wasp/packages/authentication"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/database"
	"github.com/iotaledger/wasp/packages/dkg"
	"github.com/iotaledger/wasp/packages/evm/jsonrpc"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/metrics"
	"github.com/iotaledger/wasp/packages/origin"
	"github.com/iotaledger/wasp/packages/publisher"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/users"
	"github.com/iotaledger/wasp/packages/vm/core/evm/emulator"
	"github.com/iotaledger/wasp/packages/webapi"
	"github.com/iotaledger/wasp/packages/webapi/apierrors"
	"github.com/iotaledger/wasp/packages/webapi/routes"
	"github.com/iotaledger/wasp/packages/webapi/websocket"
	"github.com/iotaledger/wasp/tools/wasp-cli/log"
)

var listenAddress string = ":9090"

func main() {
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Run:   start,
		Use:   "wasp-solo",
		Short: "wasp-solo emulates a Wasp node with Solo as L1",
		Long: fmt.Sprintf(`wasp-solo emulates a Wasp node with Solo as in-memory L1 ledger.

wasp-solo does the following:

- Starts an ISC chain in a Solo environment
- Initializes 10 non-ethereum and 10 ethereum accounts with on-chain funds
  (private keys and addresses printed after init)
- Starts a webapi server at port 9090

Note: chain data is stored in-memory and will be lost upon termination.
`,
		),
	}

	log.Init(cmd)
	cmd.PersistentFlags().StringVarP(&listenAddress, "listen", "l", ":9090", "listen address")

	err := cmd.Execute()
	log.Check(err)
}

func initChain(env *solo.Solo) *solo.Chain {
	chainOwner, chainOwnerAddr := env.NewKeyPairWithFunds(env.NewSeedFromIndex(0))
	chain, _ := env.NewChainExt(chainOwner, 1*isc.Million, 0, "wasp-solo", dict.Dict{
		origin.ParamChainOwner:      isc.NewAgentID(chainOwnerAddr).Bytes(),
		origin.ParamEVMChainID:      codec.Uint16.Encode(1074),
		origin.ParamBlockKeepAmount: codec.Int32.Encode(emulator.BlockKeepAll),
		origin.ParamWaspVersion:     codec.String.Encode(app.Version),
	})
	return chain
}

func createL1Accounts(chain *solo.Chain) []iotago.Address {
	log.Printf("creating accounts with funds...\n")
	accounts := []iotago.Address{chain.OriginatorAddress}
	for i := 1; i < 10; i++ { // index 0 is chain owner
		seed := chain.Env.NewSeedFromIndex(i)
		kp := cryptolib.KeyPairFromSeed(*seed)
		_, err := chain.Env.GetFundsFromFaucet(kp.Address())
		log.Check(err)
		log.Check(chain.DepositAssetsToL2(
			isc.NewAssetsBaseTokens(chain.Env.L1BaseTokens(kp.Address())/2),
			kp,
		))
		accounts = append(accounts, kp.GetPublicKey().AsEd25519Address())
	}
	return accounts
}

func printL1Accounts(accounts []iotago.Address, hrp iotago.NetworkPrefix) {
	header := []string{"index", "address"}
	var rows [][]string
	for i, account := range accounts {
		rows = append(rows, []string{fmt.Sprintf("%d", i), account.Bech32(hrp)})
	}
	log.PrintTable(header, rows)
}

func createEthereumAccounts(chain *solo.Chain) []*ecdsa.PrivateKey {
	log.Printf("creating Ethereum accounts with funds...\n")
	var accounts []*ecdsa.PrivateKey
	for i := 0; i < len(solo.EthereumAccounts); i++ {
		pk, _ := chain.EthereumAccountByIndexWithL2Funds(i)
		accounts = append(accounts, pk)
	}
	return accounts
}

func printEthereumAccounts(accounts []*ecdsa.PrivateKey) {
	header := []string{"private key", "address"}
	var rows [][]string
	for _, pk := range accounts {
		addr := crypto.PubkeyToAddress(pk.PublicKey)
		rows = append(rows, []string{hex.EncodeToString(crypto.FromECDSA(pk)), addr.String()})
	}
	log.PrintTable(header, rows)
}

// TODO: duplicated code from components/webapi
func newEcho(debug bool, log hivelog.Logger) *echo.Echo {
	e := httpserver.NewEcho(log, nil, debug)
	e.HTTPErrorHandler = apierrors.HTTPErrorHandler()
	webapi.ConfirmedStateLagThreshold = 2
	authentication.DefaultJWTDuration = 24 * time.Hour
	e.Pre(middleware.RemoveTrailingSlash())
	e.Use(webapi.MiddlewareUnescapePath)
	e.Use(middleware.BodyLimit("2M"))
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: `${time_rfc3339_nano} ${remote_ip} ${method} ${uri} ${status} error="${error}"` + "\n",
	}))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowHeaders:     []string{"*"},
		AllowMethods:     []string{"*"},
		AllowCredentials: true,
	}))
	return e
}

// TODO: duplicated code from components/webapi
func createEchoSwagger(e *echo.Echo, version string) echoswagger.ApiRoot {
	echoSwagger := echoswagger.New(e, "/doc", &echoswagger.Info{
		Title:       "Wasp API",
		Description: "REST API for the Wasp node",
		Version:     version,
	})
	echoSwagger.SetRequestContentType(echo.MIMEApplicationJSON)
	echoSwagger.SetResponseContentType(echo.MIMEApplicationJSON)
	return echoSwagger
}

// TODO: duplicated code from components/webapi
func initEcho(debug bool, logger hivelog.Logger) echoswagger.ApiRoot {
	e := newEcho(debug, logger)
	echoSwagger := createEchoSwagger(e, app.Version)
	if debug {
		echoSwagger.Echo().Use(middleware.BodyDump(func(c echo.Context, reqBody, resBody []byte) {
			logger.LogDebugf("API Dump: Request=%q, Response=%q", reqBody, resBody)
		}))
	}
	return echoSwagger
}

func initWsService(logger hivelog.Logger, pub *publisher.Publisher, l1 iotago.APIProvider) (*websocket.Service, *websockethub.Hub) {
	const (
		broadcastQueueSize            = 20000
		clientSendChannelSize         = 1000
		maxWebsocketMessageSize int64 = 510
	)

	websocketOptions := websocketserver.AcceptOptions{
		InsecureSkipVerify: true,
		// Disable compression due to incompatibilities with the latest Safari browsers:
		// https://github.com/tilt-dev/tilt/issues/4746
		CompressionMode: websocketserver.CompressionDisabled,
	}

	hub := websockethub.NewHub(logger, &websocketOptions, broadcastQueueSize, clientSendChannelSize, maxWebsocketMessageSize)

	wsService := websocket.NewWebsocketService(logger, hub, []publisher.ISCEventType{
		publisher.ISCEventKindNewBlock,
		publisher.ISCEventKindReceipt,
		publisher.ISCEventIssuerVM,
		publisher.ISCEventKindBlockEvents,
	}, pub, l1.LatestAPI())

	return wsService, hub
}

func start(cmd *cobra.Command, args []string) {
	seed := cryptolib.SeedFromBytes(lo.Must(hexutil.DecodeHex("0xffa736fb5373da7bf8b8c97e73157300a529cb7e37c48f3b8ce0ec3cb556e509")))

	soloCtx := &soloContext{}
	defer soloCtx.cleanupAll()

	env := solo.New(soloCtx, &solo.InitOptions{
		Debug: log.DebugFlag,
		Seed:  seed,
	})

	initDKShare(env)

	chain := initChain(env)
	chain.FakeCommitteeInfo = getCommitteeInfo
	chain.FakeChainNodes = getChainNodes

	l1Accounts := createL1Accounts(chain)
	ethAccounts := createEthereumAccounts(chain)

	jsonrpcParams := jsonrpc.ParametersDefault()
	jsonrpcParams.Accounts = ethAccounts

	echoSwagger := initEcho(
		log.DebugFlag,
		env.Log().NewChildLogger("webapi"),
	)

	wsService, wsHub := initWsService(
		env.Log().NewChildLogger("websocket"),
		env.Publisher(),
		env.L1APIProvider(),
	)

	userManager := users.NewUserManager((func(users []*users.User) error {
		return nil
	}))
	userManager.AddUser(&users.User{
		Name: "wasp",
	})

	chainMetrics := metrics.NewChainMetricsProvider()

	netProvider := networkProvider(env)
	node, err := dkg.NewNode(
		peer,
		netProvider,
		dkShareRegistryProvider,
		env.Log().NewChildLogger("dkg"),
	)
	log.Check(err)

	evmIndexDBProvider := func() (*database.Database, error) {
		return database.DatabaseWithDefaultSettings("", true, hivedb.EngineMapDB, false)
	}

	webapi.Init(
		env.Log().NewChildLogger("webapi"),
		echoSwagger,
		app.Version,
		nil, // *Configuration -- /node/config will fail
		netProvider,
		trustedNetworkManager(env),
		userManager,
		chainRecordRegistryProvider(env),
		dkShareRegistryProvider,
		nodeIdentityProvider(env),
		chainsProvider(env),
		func() *dkg.Node { return node },
		nil, // *ShutdownHandler -- /node/shutdown will fail
		chainMetrics,
		authentication.AuthConfiguration{
			Scheme:    authentication.AuthNone,
			JWTConfig: authentication.JWTAuthConfiguration{Duration: 24 * time.Hour},
		},
		300*time.Second, // APICacheTTL,
		wsService,
		evmIndexDBProvider,
		env.Publisher(),
		jsonrpcParams,
		env.L1APIProvider().LatestAPI(),
		env.TokenInfo(),
	)

	l1ApiInit(env, echoSwagger.Echo())
	l1FaucetInit(env, echoSwagger.Echo())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		unhook := wsService.EventHandler().AttachToEvents()
		defer unhook()
		wsHub.Run(ctx)
	}()

	hrp := env.L1APIProvider().CommittedAPI().ProtocolParameters().Bech32HRP()
	log.Printf("\n")
	log.Printf("ChainID: %s\n", chain.ChainID.Bech32(hrp))
	log.Printf("\nAccounts (first is chain owner):\n")
	printL1Accounts(l1Accounts, hrp)
	log.Printf("\nEthereum accounts:\n")
	printEthereumAccounts(ethAccounts)
	log.Printf("\n")
	addr := listenAddress
	if listenAddress[0] == ':' {
		addr = "http://localhost" + listenAddress
	}
	log.Printf("wasp-cli configuration\n")
	log.Printf("----------------------\n")
	log.Printf("wasp-cli wasp add 0 %s\n", addr)
	log.Printf("wasp-cli set l1.apiaddress %s/l1api\n", addr)
	log.Printf("wasp-cli set l1.faucetaddress %s/l1faucet\n", addr)
	log.Printf("wasp-cli set wallet.seed %s\n", hexutil.EncodeHex(seed[:]))
	log.Printf("wasp-cli chain add testchain %s\n", chain.ChainID.AsAddress().Bech32(hrp))
	log.Printf("\n")

	log.Printf("Metamask configuration\n")
	log.Printf("----------------------\n")
	log.Printf("Chain ID: %d\n", chain.EVM().ChainID())
	log.Printf("RPC URL: %s/v%d/chains/%s/%s\n", addr, webapi.APIVersion, chain.ChainID.Bech32(hrp), routes.EVMJsonRPCPathSuffix)
	log.Printf("Websocket: %s/v%d/chains/%s/%s\n", strings.Replace(addr, "http:", "ws:", 1), webapi.APIVersion, chain.ChainID.Bech32(hrp), routes.EVMJsonWebSocketPathSuffix)
	log.Printf("\n")

	if err := echoSwagger.Echo().Start(listenAddress); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Check(err)
	}
}
