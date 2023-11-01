package nodeconn

import (
	"fmt"
	"sort"

	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/serializer/v2"
	inx "github.com/iotaledger/inx/go"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
)

func getAliasID(outputID iotago.OutputID, anchorOutput *iotago.AnchorOutput) iotago.AccountID {
	if anchorOutput.AccountID.Empty() {
		return iotago.AliasIDFromOutputID(outputID)
	}

	return anchorOutput.AccountID
}

func outputInfoFromINXOutput(output *inx.LedgerOutput) (*isc.OutputInfo, error) {
	outputID := output.UnwrapOutputID()

	iotaOutput, err := output.UnwrapOutput(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return nil, err
	}

	return isc.NewOutputInfo(outputID, iotaOutput, iotago.TransactionID{}), nil
}

func outputInfoFromINXSpent(spent *inx.LedgerSpent) (*isc.OutputInfo, error) {
	outputInfo, err := outputInfoFromINXOutput(spent.GetOutput())
	if err != nil {
		return nil, err
	}

	outputInfo.TransactionIDSpent = spent.UnwrapTransactionIDSpent()
	return outputInfo, nil
}

func unwrapOutputs(outputs []*inx.LedgerOutput) ([]*isc.OutputInfo, error) {
	result := make([]*isc.OutputInfo, len(outputs))

	for i := range outputs {
		outputInfo, err := outputInfoFromINXOutput(outputs[i])
		if err != nil {
			return nil, err
		}
		result[i] = outputInfo
	}

	return result, nil
}

func unwrapSpents(spents []*inx.LedgerSpent) ([]*isc.OutputInfo, error) {
	result := make([]*isc.OutputInfo, len(spents))

	for i := range spents {
		outputInfo, err := outputInfoFromINXSpent(spents[i])
		if err != nil {
			return nil, err
		}
		result[i] = outputInfo
	}

	return result, nil
}

// wasOutputIDConsumedBefore checks recursively if "targetOutputID" was consumed before "outputID"
// by walking the consumed outputs of the transaction that created "outputID".
func wasOutputIDConsumedBefore(consumedOutputsMapByTransactionID map[iotago.TransactionID]map[iotago.OutputID]struct{}, targetOutputID iotago.OutputID, outputID iotago.OutputID) bool {
	consumedOutputs, exists := consumedOutputsMapByTransactionID[outputID.TransactionID()]
	if !exists {
		// if the transaction that created the "outputID" was not part of the milestone, the "outputID" was consumed before "targetOutputID"
		return false
	}

	for consumedOutput := range consumedOutputs {
		if consumedOutput == targetOutputID {
			// we found the "targetOutputID" in the past of "outputID"
			return true
		}

		// walk all consumed outputs of that transaction recursively
		if wasOutputIDConsumedBefore(consumedOutputsMapByTransactionID, targetOutputID, consumedOutput) {
			return true
		}
	}

	// we didn't find the "targetOutputID" in the past of "outputID"
	return false
}

func sortAnchorOutputsOfChain(trackedChainAnchorOutputsCreated []*isc.OutputInfo, trackedAnchorOutputsConsumedMapByTransactionID map[iotago.TransactionID]map[iotago.OutputID]struct{}) error {
	var innerErr error

	sort.SliceStable(trackedChainAnchorOutputsCreated, func(i, j int) bool {
		outputInfo1 := trackedChainAnchorOutputsCreated[i]
		outputInfo2 := trackedChainAnchorOutputsCreated[j]

		anchorOutput1 := outputInfo1.Output.(*iotago.AnchorOutput)
		anchorOutput2 := outputInfo2.Output.(*iotago.AnchorOutput)

		// check if state indexes are equal.
		if anchorOutput1.StateIndex != anchorOutput2.StateIndex {
			return anchorOutput1.StateIndex < anchorOutput2.StateIndex
		}

		outputID1 := outputInfo1.OutputID
		outputID2 := outputInfo2.OutputID

		// in case of a governance transition, the state index is equal.
		if !outputInfo1.Consumed() {
			if !outputInfo2.Consumed() {
				// this should never happen because there can't be two alias outputs with the same alias ID that are unspent.
				innerErr = fmt.Errorf("two unspent alias outputs with same AccountID found (Output1: %s, Output2: %s", outputID1.ToHex(), outputID2.ToHex())
			}
			return false
		}

		if !outputInfo2.Consumed() {
			// first output was consumed, second was not, so first is before second.
			return true
		}

		// we need to figure out the order in which they were consumed (recursive).
		if wasOutputIDConsumedBefore(trackedAnchorOutputsConsumedMapByTransactionID, outputID1, outputID2) {
			return true
		}

		if wasOutputIDConsumedBefore(trackedAnchorOutputsConsumedMapByTransactionID, outputID2, outputID1) {
			return false
		}

		innerErr = fmt.Errorf("two consumed alias outputs with same AccountID found, but ordering is unclear (Output1: %s, Output2: %s", outputID1.ToHex(), outputID2.ToHex())
		return false
	})

	return innerErr
}

func getAliasIDAnchorOutput(outputInfo *isc.OutputInfo) iotago.AccountID {
	if outputInfo.Output.Type() != iotago.OutputAlias {
		return iotago.AccountID{}
	}

	return getAliasID(outputInfo.OutputID, outputInfo.Output.(*iotago.AnchorOutput))
}

func getAliasIDOtherOutputs(output iotago.Output) iotago.AccountID {
	var addressToCheck iotago.Address
	switch output.Type() {
	case iotago.OutputBasic:
		addressToCheck = output.(*iotago.BasicOutput).Ident()

	case iotago.OutputAlias:
		// TODO: chains can't own other alias outputs for now
		return iotago.AccountID{}

	case iotago.OutputFoundry:
		addressToCheck = output.(*iotago.FoundryOutput).Ident()

	case iotago.OutputNFT:
		addressToCheck = output.(*iotago.NFTOutput).Ident()

	default:
		panic(fmt.Errorf("%w: type %d", iotago.ErrUnknownOutputType, output.Type()))
	}

	if addressToCheck.Type() != iotago.AddressAccount {
		// output is not owned by an account address => ignore it
		// nested ownerships are also ignored (Chain owns NFT that owns NFT's etc).
		return iotago.AccountID{}
	}

	return addressToCheck.(*iotago.AccountAddress).AccountID()
}

// filterAndSortAnchorOutputs filters and groups all alias outputs by chain ID and then sorts them,
// because they could have been transitioned several times in the same milestone. applying the alias outputs to the consensus
// we need to apply them in correct order.
// chainsLock needs to be read locked outside
func filterAndSortAnchorOutputs(chainsMap *shrinkingmap.ShrinkingMap[isc.ChainID, *ncChain], ledgerUpdate *ledgerUpdate) (map[isc.ChainID][]*isc.OutputInfo, map[iotago.OutputID]struct{}, error) {
	// filter and group "created alias outputs" by chain ID and also remember the tracked outputs
	trackedAnchorOutputsCreatedMapByOutputID := make(map[iotago.OutputID]struct{})
	trackedAnchorOutputsCreatedMapByChainID := make(map[isc.ChainID][]*isc.OutputInfo)
	for outputID := range ledgerUpdate.outputsCreatedMap {
		outputInfo := ledgerUpdate.outputsCreatedMap[outputID]

		aliasID := getAliasIDAnchorOutput(outputInfo)
		if aliasID.Empty() {
			continue
		}

		chainID := isc.ChainIDFromAliasID(aliasID)

		// only allow tracked chains
		if !chainsMap.Has(chainID) {
			continue
		}

		trackedAnchorOutputsCreatedMapByOutputID[outputInfo.OutputID] = struct{}{}

		if _, exists := trackedAnchorOutputsCreatedMapByChainID[chainID]; !exists {
			trackedAnchorOutputsCreatedMapByChainID[chainID] = make([]*isc.OutputInfo, 0)
		}

		trackedAnchorOutputsCreatedMapByChainID[chainID] = append(trackedAnchorOutputsCreatedMapByChainID[chainID], outputInfo)
	}

	// create a map for faster lookups of output IDs that were spent by a transaction ID.
	// this is needed to figure out the correct ordering of alias outputs in case of governance transitions.
	trackedAnchorOutputsConsumedMapByTransactionID := make(map[iotago.TransactionID]map[iotago.OutputID]struct{})
	for outputID := range ledgerUpdate.outputsConsumedMap {
		outputInfo := ledgerUpdate.outputsConsumedMap[outputID]

		aliasID := getAliasIDAnchorOutput(outputInfo)
		if aliasID.Empty() {
			continue
		}

		chainID := isc.ChainIDFromAliasID(aliasID)

		// only allow tracked chains
		if !chainsMap.Has(chainID) {
			continue
		}

		if _, exists := trackedAnchorOutputsConsumedMapByTransactionID[outputInfo.TransactionIDSpent]; !exists {
			trackedAnchorOutputsConsumedMapByTransactionID[outputInfo.TransactionIDSpent] = make(map[iotago.OutputID]struct{})
		}

		if _, exists := trackedAnchorOutputsConsumedMapByTransactionID[outputInfo.TransactionIDSpent][outputInfo.OutputID]; !exists {
			trackedAnchorOutputsConsumedMapByTransactionID[outputInfo.TransactionIDSpent][outputInfo.OutputID] = struct{}{}
		}
	}

	for chainID := range trackedAnchorOutputsCreatedMapByChainID {
		if err := sortAnchorOutputsOfChain(
			trackedAnchorOutputsCreatedMapByChainID[chainID],
			trackedAnchorOutputsConsumedMapByTransactionID,
		); err != nil {
			return nil, nil, err
		}
	}

	return trackedAnchorOutputsCreatedMapByChainID, trackedAnchorOutputsCreatedMapByOutputID, nil
}

// chainsLock needs to be read locked
func filterOtherOutputs(
	chainsMap *shrinkingmap.ShrinkingMap[isc.ChainID, *ncChain],
	outputsCreatedMap map[iotago.OutputID]*isc.OutputInfo,
	trackedAnchorOutputsCreatedMapByOutputID map[iotago.OutputID]struct{},
) map[isc.ChainID][]*isc.OutputInfo {
	otherOutputsCreatedByChainID := make(map[isc.ChainID][]*isc.OutputInfo)

	// we need to filter all other output types in case they were consumed in the same milestone.
	for outputID := range outputsCreatedMap {
		outputInfo := outputsCreatedMap[outputID]

		if _, exists := trackedAnchorOutputsCreatedMapByOutputID[outputInfo.OutputID]; exists {
			// output will already be applied
			continue
		}

		if outputInfo.Consumed() {
			// output is not an important alias output that belongs to a chain,
			// and it was already consumed in the same milestone => ignore it
			continue
		}

		aliasID := getAliasIDOtherOutputs(outputInfo.Output)
		if aliasID.Empty() {
			continue
		}

		chainID := isc.ChainIDFromAliasID(aliasID)

		// allow only tracked chains
		if !chainsMap.Has(chainID) {
			continue
		}

		if _, exists := otherOutputsCreatedByChainID[chainID]; !exists {
			otherOutputsCreatedByChainID[chainID] = make([]*isc.OutputInfo, 0)
		}

		// add the output to the tracked chain
		otherOutputsCreatedByChainID[chainID] = append(otherOutputsCreatedByChainID[chainID], outputInfo)
	}

	return otherOutputsCreatedByChainID
}
