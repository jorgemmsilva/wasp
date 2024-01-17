package chaintypes

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/iotaledger/hive.go/log"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"

	"github.com/iotaledger/wasp/packages/chain/mempool/mempooltypes"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/metrics"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/state/indexedstore"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/processors"
)

type Chain interface {
	ChainCore
	ChainRequests
	// This is invoked when a node owner updates the chain configuration,
	// possibly to update the per-node accessNode list.
	ConfigUpdated(accessNodesPerNode []*cryptolib.PublicKey)
	// This is invoked when the accessMgr determines the nodes which
	// consider this node as an access node for this chain. The chain
	// can query the nodes for blocks, etc. NOTE: servers = access⁻¹
	ServersUpdated(serverNodes []*cryptolib.PublicKey)
	// Metrics and the current descriptive state.
	GetChainMetrics() *metrics.ChainMetrics
	GetConsensusPipeMetrics() ConsensusPipeMetrics // TODO: Review this.
	GetConsensusWorkflowStatus() ConsensusWorkflowStatus
	GetMempoolContents() io.Reader
}

type StateFreshness byte

const (
	ActiveOrCommittedState StateFreshness = iota // ActiveState, if exist; Confirmed state otherwise.
	ActiveState                                  // The state the chain build next TX on, can be ahead of ConfirmedState.
	ConfirmedState                               // The state confirmed on L1.
)

func (sf StateFreshness) String() string {
	switch sf {
	case ActiveOrCommittedState:
		return "ActiveOrCommittedState"
	case ActiveState:
		return "ActiveState"
	case ConfirmedState:
		return "ConfirmedState"
	default:
		return fmt.Sprintf("StateFreshness=%v", int(sf))
	}
}

type ChainCore interface {
	ID() isc.ChainID
	// Returns the current latest confirmed anchor output and the active one.
	// The active AO can be ahead of the confirmed one by several blocks.
	// Both values can be nil, if the node haven't received an output from
	// L1 yet (after a restart or a chain activation).
	LatestChainOutputs(freshness StateFreshness) (*isc.ChainOutputs, error)
	LatestState(freshness StateFreshness) (state.State, error)
	GetCommitteeInfo() *CommitteeInfo // TODO: Review, maybe we can reorganize the CommitteeInfo structure.
	Store() indexedstore.IndexedStore // Use LatestState whenever possible. That will work faster.
	Processors() *processors.Cache
	GetChainNodes() []peering.PeerStatusProvider     // CommitteeNodes + AccessNodes
	GetCandidateNodes() []*governance.AccessNodeInfo // All the current candidates.
	L1APIProvider() iotago.APIProvider
	TokenInfo() *api.InfoResBaseToken
	Log() log.Logger
}

type ChainRequests interface {
	ReceiveOffLedgerRequest(request isc.OffLedgerRequest, sender *cryptolib.PublicKey) error
	AwaitRequestProcessed(ctx context.Context, requestID isc.RequestID, confirmed bool) <-chan *blocklog.RequestReceipt
}

type ConsensusPipeMetrics interface { // TODO: Review it.
	GetEventStateTransitionMsgPipeSize() int
	GetEventPeerLogIndexMsgPipeSize() int
	GetEventACSMsgPipeSize() int
	GetEventVMResultMsgPipeSize() int
	GetEventTimerMsgPipeSize() int
}

type ConsensusWorkflowStatus interface { // TODO: Review it.
	IsStateReceived() bool
	IsBatchProposalSent() bool
	IsConsensusBatchKnown() bool
	IsVMStarted() bool
	IsVMResultSigned() bool
	IsTransactionFinalized() bool
	IsTransactionPosted() bool
	IsTransactionSeen() bool
	IsInProgress() bool
	GetBatchProposalSentTime() time.Time
	GetConsensusBatchKnownTime() time.Time
	GetVMStartedTime() time.Time
	GetVMResultSignedTime() time.Time
	GetTransactionFinalizedTime() time.Time
	GetTransactionPostedTime() time.Time
	GetTransactionSeenTime() time.Time
	GetCompletedTime() time.Time
	GetCurrentStateIndex() uint32
}

type CommitteeInfo struct {
	Address       iotago.Address
	Size          uint16
	Quorum        uint16
	QuorumIsAlive bool
	PeerStatus    []*PeerStatus
}

type PeerStatus struct {
	Name       string
	Index      uint16
	PubKey     *cryptolib.PublicKey
	PeeringURL string
	Connected  bool
}

// Implementation of this interface will receive events in the chain.
// Initial intention was to provide info to the published/WebSocket endpoint.
// All the function MUST NOT BLOCK.
type ChainListener interface {
	mempooltypes.ChainListener
	AccessNodesUpdated(chainID isc.ChainID, accessNodes []*cryptolib.PublicKey)
	ServerNodesUpdated(chainID isc.ChainID, serverNodes []*cryptolib.PublicKey)
}
