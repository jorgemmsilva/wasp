package services

import (
	"net/http"
	"sync"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/labstack/echo/v4"

	hivedb "github.com/iotaledger/hive.go/kvstore/database"
	"github.com/iotaledger/hive.go/logger"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/chains"
	"github.com/iotaledger/wasp/packages/database"
	"github.com/iotaledger/wasp/packages/evm/jsonrpc"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/metrics"

	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/publisher"
	"github.com/iotaledger/wasp/packages/webapi/interfaces"
)

type chainServer struct {
	backend *jsonrpc.WaspEVMBackend
	rpc     *rpc.Server
}

type EVMService struct {
	evmBackendMutex sync.Mutex
	evmChainServers map[isc.ChainID]*chainServer

	baseTokenInfo   api.InfoResBaseToken
	chainsProvider  chains.Provider
	chainService    interfaces.ChainService
	networkProvider peering.NetworkProvider
	publisher       *publisher.Publisher
	indexDbPath     string
	l1API           iotago.API
	metrics         *metrics.ChainMetricsProvider
	jsonrpcParams   *jsonrpc.Parameters
	log             *logger.Logger
}

func NewEVMService(
	baseTokenInfo api.InfoResBaseToken,
	chainsProvider chains.Provider,
	chainService interfaces.ChainService,
	l1API iotago.API,
	networkProvider peering.NetworkProvider,
	pub *publisher.Publisher,
	indexDbPath string,
	metrics *metrics.ChainMetricsProvider,
	jsonrpcParams *jsonrpc.Parameters,
	log *logger.Logger,
) interfaces.EVMService {
	return &EVMService{
		baseTokenInfo:   baseTokenInfo,
		chainsProvider:  chainsProvider,
		chainService:    chainService,
		evmChainServers: map[isc.ChainID]*chainServer{},
		evmBackendMutex: sync.Mutex{},
		l1API:           l1API,
		networkProvider: networkProvider,
		publisher:       pub,
		indexDbPath:     indexDbPath,
		metrics:         metrics,
		jsonrpcParams:   jsonrpcParams,
		log:             log,
	}
}

func (e *EVMService) getEVMBackend(chainID isc.ChainID) (*chainServer, error) {
	e.evmBackendMutex.Lock()
	defer e.evmBackendMutex.Unlock()

	if e.evmChainServers[chainID] != nil {
		return e.evmChainServers[chainID], nil
	}

	chain, err := e.chainService.GetChainByID(chainID)
	if err != nil {
		return nil, err
	}

	nodePubKey := e.networkProvider.Self().PubKey()
	backend := jsonrpc.NewWaspEVMBackend(chain, nodePubKey, e.baseTokenInfo)

	db, err := database.DatabaseWithDefaultSettings(e.indexDbPath, true, hivedb.EngineRocksDB, false)
	if err != nil {
		panic(err)
	}

	// TODO: <lmoe> Validate if this DB approach is correct.
	srv, err := jsonrpc.NewServer(
		jsonrpc.NewEVMChain(backend, e.publisher, e.chainsProvider().IsArchiveNode(), db.KVStore(), e.log.Named("EVMChain")),
		jsonrpc.NewAccountManager(nil),
		e.metrics.GetChainMetrics(chainID).WebAPI,
		e.jsonrpcParams,
		func() iotago.API {
			return e.l1API
		},
	)
	if err != nil {
		return nil, err
	}

	e.evmChainServers[chainID] = &chainServer{
		backend: backend,
		rpc:     srv,
	}

	return e.evmChainServers[chainID], nil
}

func (e *EVMService) HandleJSONRPC(chainID isc.ChainID, request *http.Request, response *echo.Response) error {
	evmServer, err := e.getEVMBackend(chainID)
	if err != nil {
		return err
	}

	evmServer.rpc.ServeHTTP(response, request)

	return nil
}

func (e *EVMService) HandleWebsocket(chainID isc.ChainID, request *http.Request, response *echo.Response) error {
	evmServer, err := e.getEVMBackend(chainID)
	if err != nil {
		return err
	}

	allowedOrigins := []string{"*"}
	evmServer.rpc.WebsocketHandler(allowedOrigins).ServeHTTP(response, request)

	return nil
}
