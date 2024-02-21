package testchain

import (
	"errors"
	"sync"

	"github.com/samber/lo"

	"github.com/iotaledger/hive.go/log"
	"github.com/iotaledger/hive.go/runtime/event"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/origin"
	"github.com/iotaledger/wasp/packages/testutil"
)

type MockedLedger struct {
	latestOutputID                 iotago.OutputID
	outputs                        map[iotago.OutputID]*iotago.AnchorOutput
	txIDs                          map[iotago.TransactionID]bool
	publishTransactionAllowedFun   func(tx *iotago.SignedTransaction) bool
	pullLatestOutputAllowed        bool
	pullTxInclusionStateAllowedFun func(iotago.TransactionID) bool
	pullOutputByIDAllowedFun       func(iotago.OutputID) bool
	pushOutputToNodesNeededFun     func(*iotago.SignedTransaction, iotago.OutputID, iotago.Output) bool
	stateOutputHandlerFuns         map[string]func(iotago.OutputID, iotago.Output)
	outputHandlerFuns              map[string]func(iotago.OutputID, iotago.Output)
	inclusionStateEvents           map[string]*event.Event2[iotago.TransactionID, string]
	mutex                          sync.RWMutex
	log                            log.Logger
}

func NewMockedLedger(stateAddress iotago.Address, log log.Logger) (*MockedLedger, isc.ChainID) {
	originOutput := &iotago.AnchorOutput{
		Amount: testutil.L1API.ProtocolParameters().TokenSupply(),
		UnlockConditions: iotago.AnchorOutputUnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{Address: stateAddress},
			&iotago.GovernorAddressUnlockCondition{Address: stateAddress},
		},
		Features: iotago.AnchorOutputFeatures{
			&iotago.SenderFeature{
				Address: stateAddress,
			},
			&iotago.StateMetadataFeature{
				Entries: map[iotago.StateMetadataFeatureEntriesKey]iotago.StateMetadataFeatureEntriesValue{
					"": testutil.DummyStateMetadata(origin.L1Commitment(0, nil, 0, testutil.TokenInfo, testutil.L1API)).Bytes(),
				},
			},
		},
	}
	outputID := getOriginOutputID()
	chainID := isc.ChainIDFromAnchorID(iotago.AnchorIDFromOutputID(outputID))
	originOutput.AnchorID = chainID.AsAnchorID() // NOTE: not very correct: origin output's AccountID should be empty; left here to make mocking transitions easier
	outputs := make(map[iotago.OutputID]*iotago.AnchorOutput)
	outputs[outputID] = originOutput
	ret := &MockedLedger{
		latestOutputID:         outputID,
		outputs:                outputs,
		txIDs:                  make(map[iotago.TransactionID]bool),
		stateOutputHandlerFuns: make(map[string]func(iotago.OutputID, iotago.Output)),
		outputHandlerFuns:      make(map[string]func(iotago.OutputID, iotago.Output)),
		inclusionStateEvents:   make(map[string]*event.Event2[iotago.TransactionID, string]),
		log:                    log.NewChildLogger("ml-" + chainID.Bech32(testutil.L1API.ProtocolParameters().Bech32HRP())[2:8]),
	}
	ret.SetPublishStateTransactionAllowed(true)
	ret.SetPublishGovernanceTransactionAllowed(true)
	ret.SetPullLatestOutputAllowed(true)
	ret.SetPullTxInclusionStateAllowed(true)
	ret.SetPullOutputByIDAllowed(true)
	ret.SetPushOutputToNodesNeeded(true)
	return ret, chainID
}

func (mlT *MockedLedger) Register(nodeID string, stateOutputHandler, outputHandler func(iotago.OutputID, iotago.Output)) {
	mlT.mutex.Lock()
	defer mlT.mutex.Unlock()

	_, ok := mlT.outputHandlerFuns[nodeID]
	if ok {
		mlT.log.LogPanicf("Output handler for node %v already registered", nodeID)
	}
	mlT.stateOutputHandlerFuns[nodeID] = stateOutputHandler
	mlT.outputHandlerFuns[nodeID] = outputHandler
}

func (mlT *MockedLedger) Unregister(nodeID string) {
	mlT.mutex.Lock()
	defer mlT.mutex.Unlock()

	delete(mlT.stateOutputHandlerFuns, nodeID)
	delete(mlT.outputHandlerFuns, nodeID)
}

func (mlT *MockedLedger) PublishTransaction(tx *iotago.SignedTransaction) error {
	mlT.mutex.Lock()
	defer mlT.mutex.Unlock()

	if mlT.publishTransactionAllowedFun(tx) {
		mlT.log.LogDebugf("Publishing transaction allowed, transaction has %v inputs, %v outputs, %v unlock blocks",
			len(lo.Must(tx.Transaction.Inputs())), len(tx.Transaction.Outputs), len(tx.Unlocks))
		txID, err := tx.Transaction.ID()
		if err != nil {
			mlT.log.LogPanicf("Publishing transaction: cannot calculate transaction id: %v", err)
		}
		mlT.log.LogDebugf("Publishing transaction: transaction id is %s", txID.ToHex())
		mlT.txIDs[txID] = true
		for index, output := range tx.Transaction.Outputs {
			anchorOutput, ok := output.(*iotago.AnchorOutput)
			outputID := iotago.OutputIDFromTransactionIDAndIndex(txID, uint16(index))
			mlT.log.LogDebugf("Publishing transaction: outputs[%v] has id %v", index, outputID.ToHex())
			if ok {
				mlT.log.LogDebugf("Publishing transaction: outputs[%v] is anchor output", index)
				mlT.outputs[outputID] = anchorOutput
				currentLatestAnchorOutput := mlT.getAnchorOutput(mlT.latestOutputID)
				if currentLatestAnchorOutput == nil || currentLatestAnchorOutput.StateIndex < anchorOutput.StateIndex {
					mlT.log.LogDebugf("Publishing transaction: outputs[%v] is newer than current newest output (%v -> %v)",
						index, currentLatestAnchorOutput.StateIndex, anchorOutput.StateIndex)
					mlT.latestOutputID = outputID
				}
			}
			if mlT.pushOutputToNodesNeededFun(tx, outputID, output) {
				mlT.log.LogDebugf("Publishing transaction: pushing it to nodes")
				for nodeID, handler := range mlT.stateOutputHandlerFuns {
					mlT.log.LogDebugf("Publishing transaction: pushing it to node %v", nodeID)
					go handler(outputID, output)
				}
			} else {
				mlT.log.LogDebugf("Publishing transaction: pushing it to nodes not needed")
			}
		}
		return nil
	}
	return errors.New("publishing transaction not allowed")
}

func (mlT *MockedLedger) PullLatestOutput(nodeID string) {
	mlT.mutex.RLock()
	defer mlT.mutex.RUnlock()

	mlT.log.LogDebugf("Pulling latest output")
	if mlT.pullLatestOutputAllowed {
		mlT.log.LogDebugf("Pulling latest output allowed")
		output := mlT.getLatestOutput()
		mlT.log.LogDebugf("Pulling latest output: output with id %v pulled", mlT.latestOutputID.ToHex())
		handler, ok := mlT.stateOutputHandlerFuns[nodeID]
		if ok {
			go handler(mlT.latestOutputID, output)
		} else {
			mlT.log.LogPanicf("Pulling latest output: no output handler for node id %v", nodeID)
		}
	} else {
		mlT.log.LogError("Pulling latest output not allowed")
	}
}

func (mlT *MockedLedger) PullTxInclusionState(nodeID string, txID iotago.TransactionID) {
	mlT.mutex.RLock()
	defer mlT.mutex.RUnlock()

	txIDHex := txID.ToHex()
	mlT.log.LogDebugf("Pulling transaction inclusion state for ID %v", txIDHex)
	if mlT.pullTxInclusionStateAllowedFun(txID) {
		_, ok := mlT.txIDs[txID]
		var stateStr string
		if ok {
			stateStr = "included"
		} else {
			stateStr = "noTransaction"
		}
		mlT.log.LogDebugf("Pulling transaction inclusion state for ID %v: result is %v", txIDHex, stateStr)
		event, ok := mlT.inclusionStateEvents[nodeID]
		if ok {
			event.Trigger(txID, stateStr)
		} else {
			mlT.log.LogPanicf("Pulling transaction inclusion state for ID %v: no event for node id %v", txIDHex, nodeID)
		}
	} else {
		mlT.log.LogErrorf("Pulling transaction inclusion state for ID %v not allowed", txIDHex)
	}
}

func (mlT *MockedLedger) PullStateOutputByID(nodeID string, outputID iotago.OutputID) {
	mlT.mutex.RLock()
	defer mlT.mutex.RUnlock()

	outputIDHex := outputID.ToHex()

	mlT.log.LogDebugf("Pulling output by id %v", outputIDHex)
	if mlT.pullOutputByIDAllowedFun(outputID) {
		mlT.log.LogDebugf("Pulling output by id %v allowed", outputIDHex)
		anchorOutput := mlT.getAnchorOutput(outputID)
		if anchorOutput == nil {
			mlT.log.LogWarnf("Pulling output by id %v failed: output not found", outputIDHex)
			return
		}
		mlT.log.LogDebugf("Pulling output by id %v was successful", outputIDHex)
		handler, ok := mlT.stateOutputHandlerFuns[nodeID]
		if ok {
			go handler(outputID, anchorOutput)
		} else {
			mlT.log.LogPanicf("Pulling output by id %v: no output handler for node id %v", outputIDHex, nodeID)
		}
	} else {
		mlT.log.LogErrorf("Pulling output by id %v not allowed", outputIDHex)
	}
}

func (mlT *MockedLedger) GetLatestOutput() *isc.ChainOutputs {
	mlT.mutex.RLock()
	defer mlT.mutex.RUnlock()

	mlT.log.LogDebugf("Getting latest output")
	// TODO fill account outputs
	return isc.NewChainOutputs(mlT.getLatestOutput(), mlT.latestOutputID, nil, iotago.EmptyOutputID)
}

func (mlT *MockedLedger) getLatestOutput() *iotago.AnchorOutput {
	anchorOutput := mlT.getAnchorOutput(mlT.latestOutputID)
	if anchorOutput == nil {
		mlT.log.LogPanicf("Latest output with id %v not found", mlT.latestOutputID.ToHex())
	}
	return anchorOutput
}

func (mlT *MockedLedger) GetAnchorOutputByID(outputID iotago.OutputID) *iotago.AnchorOutput {
	mlT.mutex.RLock()
	defer mlT.mutex.RUnlock()

	mlT.log.LogDebugf("Getting anchor output by ID %v", outputID.ToHex())
	return mlT.getAnchorOutput(outputID)
}

func (mlT *MockedLedger) getAnchorOutput(outputID iotago.OutputID) *iotago.AnchorOutput {
	output, ok := mlT.outputs[outputID]
	if ok {
		return output
	}
	return nil
}

func (mlT *MockedLedger) SetPublishStateTransactionAllowed(flag bool) {
	mlT.SetPublishStateTransactionAllowedFun(func(*iotago.SignedTransaction) bool { return flag })
}

func (mlT *MockedLedger) SetPublishStateTransactionAllowedFun(fun func(tx *iotago.SignedTransaction) bool) {
	mlT.mutex.Lock()
	defer mlT.mutex.Unlock()

	mlT.publishTransactionAllowedFun = fun
}

func (mlT *MockedLedger) SetPublishGovernanceTransactionAllowed(flag bool) {
	mlT.SetPublishGovernanceTransactionAllowedFun(func(*iotago.SignedTransaction) bool { return flag })
}

func (mlT *MockedLedger) SetPublishGovernanceTransactionAllowedFun(fun func(tx *iotago.SignedTransaction) bool) {
	mlT.mutex.Lock()
	defer mlT.mutex.Unlock()

	mlT.publishTransactionAllowedFun = fun
}

func (mlT *MockedLedger) SetPullLatestOutputAllowed(flag bool) {
	mlT.mutex.Lock()
	defer mlT.mutex.Unlock()

	mlT.pullLatestOutputAllowed = flag
}

func (mlT *MockedLedger) SetPullTxInclusionStateAllowed(flag bool) {
	mlT.SetPullTxInclusionStateAllowedFun(func(iotago.TransactionID) bool { return flag })
}

func (mlT *MockedLedger) SetPullTxInclusionStateAllowedFun(fun func(txID iotago.TransactionID) bool) {
	mlT.mutex.Lock()
	defer mlT.mutex.Unlock()

	mlT.pullTxInclusionStateAllowedFun = fun
}

func (mlT *MockedLedger) SetPullOutputByIDAllowed(flag bool) {
	mlT.SetPullOutputByIDAllowedFun(func(iotago.OutputID) bool { return flag })
}

func (mlT *MockedLedger) SetPullOutputByIDAllowedFun(fun func(outputID iotago.OutputID) bool) {
	mlT.mutex.Lock()
	defer mlT.mutex.Unlock()

	mlT.pullOutputByIDAllowedFun = fun
}

func (mlT *MockedLedger) SetPushOutputToNodesNeeded(flag bool) {
	mlT.SetPushOutputToNodesNeededFun(func(*iotago.SignedTransaction, iotago.OutputID, iotago.Output) bool { return flag })
}

func (mlT *MockedLedger) SetPushOutputToNodesNeededFun(fun func(tx *iotago.SignedTransaction, outputID iotago.OutputID, output iotago.Output) bool) {
	mlT.mutex.Lock()
	defer mlT.mutex.Unlock()

	mlT.pushOutputToNodesNeededFun = fun
}

func getOriginOutputID() iotago.OutputID {
	return iotago.OutputID{}
}

// TODO remove, unused?
// func (mlT *MockedLedger) GetOriginOutput() *isc.ChainOutputs {
// 	mlT.mutex.RLock()
// 	defer mlT.mutex.RUnlock()

// 	outputID := getOriginOutputID()
// 	anchorOutput := mlT.getAnchorOutput(outputID)
// 	if anchorOutput == nil {
// 		return nil
// 	}
// 	return isc.NewChainOutputs(anchorOutput, outputID)
// }
