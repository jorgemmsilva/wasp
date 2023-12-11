package bp_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/chain/cons/bp"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/origin"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
	"github.com/iotaledger/wasp/packages/testutil/utxodb"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/migrations/allmigrations"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

func TestOffLedgerOrdering(t *testing.T) {
	log := testlogger.NewLogger(t)
	nodeIDs := gpa.MakeTestNodeIDs(1)
	//
	// Produce an anchor output.
	cmtKP := cryptolib.NewKeyPair()
	utxoDB := utxodb.New(testutil.L1API)
	originator := cryptolib.NewKeyPair()
	_, err := utxoDB.GetFundsFromFaucet(originator.Address())
	require.NoError(t, err)
	outputs := utxoDB.GetUnspentOutputs(originator.Address())

	originTX, _, chainID, err := origin.NewChainOriginTransaction(
		originator,
		cmtKP.Address(),
		originator.Address(),
		0,
		0,
		nil,
		outputs,
		testutil.L1API.TimeProvider().SlotFromTime(time.Now()),
		allmigrations.DefaultScheme.LatestSchemaVersion(),
		testutil.L1APIProvider,
	)
	require.NoError(t, err)
	stateAnchor, anchorOutput, err := transaction.GetAnchorFromTransaction(originTX.Transaction)
	require.NoError(t, err)
	ao0 := isc.NewChainOutputs(anchorOutput, stateAnchor.OutputID, nil, iotago.OutputID{})
	//
	// Create some requests.
	senderKP := cryptolib.NewKeyPair()
	contract := governance.Contract.Hname()
	entryPoint := governance.FuncAddCandidateNode.Hname()
	gasBudget := gas.LimitsDefault.MaxGasPerRequest
	r0 := isc.NewOffLedgerRequest(chainID, contract, entryPoint, nil, 0, gasBudget).Sign(senderKP)
	r1 := isc.NewOffLedgerRequest(chainID, contract, entryPoint, nil, 1, gasBudget).Sign(senderKP)
	r2 := isc.NewOffLedgerRequest(chainID, contract, entryPoint, nil, 2, gasBudget).Sign(senderKP)
	r3 := isc.NewOffLedgerRequest(chainID, contract, entryPoint, nil, 3, gasBudget).Sign(senderKP)
	rs := []isc.Request{r3, r1, r0, r2} // Out of order.
	//
	// Construct the batch proposal, and aggregate it.
	bp0 := bp.NewBatchProposal(
		0,
		ao0,
		util.NewFixedSizeBitVector(1).SetBits([]int{0}),
		time.Now(),
		isc.NewRandomAgentID(),
		isc.RequestRefsFromRequests(rs),
	)
	bp0.Bytes()
	abpInputs := map[gpa.NodeID][]byte{
		nodeIDs[0]: bp0.Bytes(),
	}
	abp := bp.AggregateBatchProposals(abpInputs, nodeIDs, 0, log)
	require.NotNil(t, abp)
	require.Equal(t, len(abp.DecidedRequestRefs()), len(rs))
	//
	// ...
	rndSeed := rand.New(rand.NewSource(rand.Int63()))
	randomness := hashing.PseudoRandomHash(rndSeed)
	sortedRS := abp.OrderedRequests(rs, randomness)

	for i := range sortedRS {
		for j := range sortedRS {
			if i >= j {
				continue
			}
			oflI, okI := sortedRS[i].(isc.OffLedgerRequest)
			oflJ, okJ := sortedRS[j].(isc.OffLedgerRequest)
			if !okI || !okJ {
				continue
			}
			if !oflI.SenderAccount().Equals(oflJ.SenderAccount()) {
				continue
			}
			require.Less(t, oflI.Nonce(), oflJ.Nonce(), "i=%v, j=%v", i, j)
		}
	}
}
