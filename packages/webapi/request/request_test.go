package request

import (
	"net/http"
	"testing"
	"time"

	"github.com/iotaledger/wasp/packages/chain"
	"github.com/iotaledger/wasp/packages/chains"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/state"
	util "github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/testutil/testchain"
	"github.com/iotaledger/wasp/packages/util/expiringcache"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/processors"
	"github.com/iotaledger/wasp/packages/webapi/model"
	"github.com/iotaledger/wasp/packages/webapi/routes"
	"github.com/iotaledger/wasp/packages/webapi/testutil"
	"go.uber.org/zap"
)

type mockedChain struct {
	// TODO mock chaincore is deprecated, what should be used in its place?
	*testchain.MockedChainCore
}

// chain.Chain implementation
func (*mockedChain) GetCandidateNodes() []*governance.AccessNodeInfo {
	panic("unimplemented")
}

func (*mockedChain) GetChainNodes() []peering.PeerStatusProvider {
	panic("unimplemented")
}

func (*mockedChain) GetCommitteeInfo() *chain.CommitteeInfo {
	panic("unimplemented")
}

func (*mockedChain) GetStateReader() state.Store {
	panic("unimplemented")
}

func (*mockedChain) ID() *isc.ChainID {
	panic("unimplemented")
}

func (*mockedChain) Log() *zap.SugaredLogger {
	panic("unimplemented")
}

func (*mockedChain) Processors() *processors.Cache {
	panic("unimplemented")
}

func (*mockedChain) ReceiveOffLedgerRequest(request isc.OffLedgerRequest, sender *cryptolib.PublicKey) {
	panic("unimplemented")
}

var _ chain.Chain = &mockedChain{}

// private methods

func createMockedGetChain(t *testing.T) chains.ChainProvider {
	return func(chainID *isc.ChainID) chain.Chain {
		panic("TODO revisit")
		return nil
		// chainCore := testchain.NewMockedChainCore(t, chainID, testlogger.NewLogger(t))
		// chainCore.OnOffLedgerRequest(func(msg *messages.OffLedgerRequestMsgIn) {
		// t.Logf("Offledger request %v received", msg)
		// })
		// return &mockedChain{chainCore}
	}
}

func getAccountBalanceMocked(_ chain.ChainCore, _ isc.AgentID) (*isc.FungibleTokens, error) {
	return isc.NewFungibleBaseTokens(100), nil
}

func hasRequestBeenProcessedMocked(ret bool) hasRequestBeenProcessedFn {
	return func(_ chain.ChainCore, _ isc.RequestID) (bool, error) {
		return ret, nil
	}
}

func checkNonceMocked(ch chain.ChainCore, req isc.OffLedgerRequest) error {
	return nil
}

func newMockedAPI(t *testing.T) *offLedgerReqAPI {
	return &offLedgerReqAPI{
		getChain:                createMockedGetChain(t),
		getAccountAssets:        getAccountBalanceMocked,
		hasRequestBeenProcessed: hasRequestBeenProcessedMocked(false),
		checkNonce:              checkNonceMocked,
		requestsCache:           expiringcache.New(10 * time.Second),
	}
}

func testRequest(t *testing.T, instance *offLedgerReqAPI, chainID *isc.ChainID, body interface{}, expectedStatus int) {
	testutil.CallWebAPIRequestHandler(
		t,
		instance.handleNewRequest,
		http.MethodPost,
		routes.NewRequest(":chainID"),
		map[string]string{"chainID": chainID.String()},
		body,
		nil,
		expectedStatus,
	)
}

// Tests

func TestNewRequestBase64(t *testing.T) {
	instance := newMockedAPI(t)
	chainID := isc.RandomChainID()
	body := model.OffLedgerRequestBody{Request: model.NewBytes(util.DummyOffledgerRequest(chainID).Bytes())}
	testRequest(t, instance, chainID, body, http.StatusAccepted)
}

func TestNewRequestBinary(t *testing.T) {
	instance := newMockedAPI(t)
	chainID := isc.RandomChainID()
	body := util.DummyOffledgerRequest(chainID).Bytes()
	testRequest(t, instance, chainID, body, http.StatusAccepted)
}

func TestRequestAlreadyProcessed(t *testing.T) {
	instance := newMockedAPI(t)
	instance.hasRequestBeenProcessed = hasRequestBeenProcessedMocked(true)

	chainID := isc.RandomChainID()
	body := util.DummyOffledgerRequest(chainID).Bytes()
	testRequest(t, instance, chainID, body, http.StatusBadRequest)
}

func TestWrongChainID(t *testing.T) {
	instance := newMockedAPI(t)
	body := util.DummyOffledgerRequest(isc.RandomChainID()).Bytes()
	testRequest(t, instance, isc.RandomChainID(), body, http.StatusBadRequest)
}
