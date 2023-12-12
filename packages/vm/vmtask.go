package vm

import (
	"time"

	"github.com/iotaledger/hive.go/logger"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/migrations"
	"github.com/iotaledger/wasp/packages/vm/processors"
)

// VMTask is task context (for batch of requests). It is used to pass parameters and take results
// It is assumed that all requests/inputs are unlock-able by anchorAddress of provided AnchorOutput
// at timestamp = Timestamp + len(Requests) nanoseconds
type VMTask struct {
	Processors         *processors.Cache
	Inputs             *isc.ChainOutputs
	Store              state.Store
	Requests           []isc.Request
	Timestamp          time.Time
	Entropy            hashing.HashValue
	ValidatorFeeTarget isc.AgentID
	// If EstimateGasMode is enabled, gas fee will be calculated but not charged
	EstimateGasMode bool
	// If EVMTracer is set, all requests will be executed normally up until the EVM
	// tx with the given index, which will then be executed with the given tracer.
	EVMTracer            *isc.EVMTracer
	EnableGasBurnLogging bool // for testing and Solo only

	MigrationsOverride *migrations.MigrationScheme // for testing and Solo only

	L1APIProvider iotago.APIProvider
	TokenInfo     *api.InfoResBaseToken

	Log *logger.Logger
}

type VMTaskResult struct {
	Task *VMTask

	// StateDraft is the uncommitted state resulting from the execution of the requests
	StateDraft state.StateDraft
	// RotationAddress is the next address after a rotation, or nil if there is no rotation
	RotationAddress iotago.Address
	// Transaction is the (unsigned) transaction for the next block, or the rotation transaction if RotationAddress != nil
	Transaction *iotago.Transaction
	// Unlocks contains the unlocks needed for the SignedTransaction. The first item is
	// a SignatureUnlock that should be replaced with the actual signature.
	Unlocks iotago.Unlocks
	// RequestResults contains one result for each non-skipped request
	RequestResults []*RequestResult
}

type RequestResult struct {
	// Request is the corresponding request in the task
	Request isc.Request
	// Return is the return value of the call
	Return dict.Dict
	// Receipt is the receipt produced after executing the request
	Receipt *blocklog.RequestReceipt
}

func (task *VMTask) WillProduceBlock() bool {
	return !task.EstimateGasMode && task.EVMTracer == nil
}

func (task *VMTask) FinalStateTimestamp() time.Time {
	return task.Timestamp.Add(time.Duration(len(task.Requests)+1) * time.Nanosecond)
}

func (task *VMTask) L1API() iotago.API {
	return task.L1APIProvider.APIForTime(task.Timestamp)
}
