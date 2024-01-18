package services

import (
	"context"
	"net/http"
	"sync"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/labstack/echo/v4"

	"github.com/iotaledger/hive.go/log"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
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

	websocketContextMutex sync.Mutex
	websocketContexts     map[isc.ChainID]*websocketContext

	baseTokenInfo   *api.InfoResBaseToken
	chainsProvider  chaintypes.ChainsProvider
	chainService    interfaces.ChainService
	networkProvider peering.NetworkProvider
	publisher       *publisher.Publisher
	indexDbProvider database.Provider
	l1API           iotago.API
	metrics         *metrics.ChainMetricsProvider
	jsonrpcParams   *jsonrpc.Parameters
	log             log.Logger
}

func NewEVMService(
	baseTokenInfo *api.InfoResBaseToken,
	chainsProvider chaintypes.ChainsProvider,
	chainService interfaces.ChainService,
	l1API iotago.API,
	networkProvider peering.NetworkProvider,
	pub *publisher.Publisher,
	indexDbProvider database.Provider,
	metrics *metrics.ChainMetricsProvider,
	jsonrpcParams *jsonrpc.Parameters,
	log log.Logger,
) interfaces.EVMService {
	return &EVMService{
		baseTokenInfo:         baseTokenInfo,
		chainsProvider:        chainsProvider,
		chainService:          chainService,
		evmChainServers:       map[isc.ChainID]*chainServer{},
		evmBackendMutex:       sync.Mutex{},
		websocketContexts:     map[isc.ChainID]*websocketContext{},
		websocketContextMutex: sync.Mutex{},
		l1API:                 l1API,
		networkProvider:       networkProvider,
		publisher:             pub,
		indexDbProvider:       indexDbProvider,
		metrics:               metrics,
		jsonrpcParams:         jsonrpcParams,
		log:                   log,
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
	backend := jsonrpc.NewWaspEVMBackend(chain, nodePubKey)

	db, err := e.indexDbProvider()
	if err != nil {
		panic(err)
	}

	// TODO: <lmoe> Validate if this DB approach is correct.
	srv, err := jsonrpc.NewServer(
		jsonrpc.NewEVMChain(backend, e.publisher, e.chainsProvider().IsArchiveNode(), db.KVStore(), e.log.NewChildLogger("EVMChain")),
		jsonrpc.NewAccountManager(nil),
		e.metrics.GetChainMetrics(chainID).WebAPI,
		e.jsonrpcParams,
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

func (e *EVMService) getWebsocketContext(ctx context.Context, chainID isc.ChainID) *websocketContext {
	e.websocketContextMutex.Lock()
	defer e.websocketContextMutex.Unlock()

	if e.websocketContexts[chainID] != nil {
		return e.websocketContexts[chainID]
	}

	e.websocketContexts[chainID] = newWebsocketContext(e.log, e.jsonrpcParams)
	go e.websocketContexts[chainID].runCleanupTimer(ctx)

	return e.websocketContexts[chainID]
}

func (e *EVMService) HandleWebsocket(ctx context.Context, chainID isc.ChainID, echoCtx echo.Context) error {
	evmServer, err := e.getEVMBackend(chainID)
	if err != nil {
		return err
	}

	wsContext := e.getWebsocketContext(ctx, chainID)
	websocketHandler(evmServer, wsContext, echoCtx.RealIP()).ServeHTTP(echoCtx.Response(), echoCtx.Request())
	return nil
}
