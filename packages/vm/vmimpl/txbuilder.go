package vmimpl

import (
	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/packages/vm/vmtxbuilder"
)

func (vmctx *vmContext) stateMetadata(stateCommitment *state.L1Commitment) []byte {
	stateMetadata := transaction.StateMetadata{
		Version:      transaction.StateMetadataSupportedVersion,
		L1Commitment: stateCommitment,
	}

	stateMetadata.SchemaVersion = root.NewStateReaderFromChainState(vmctx.stateDraft).GetSchemaVersion()

	// On error, the publicURL is len(0)
	govState := governance.NewStateReaderFromChainState(vmctx.stateDraft)
	stateMetadata.PublicURL, _ = govState.GetPublicURL()
	stateMetadata.GasFeePolicy = lo.Must(govState.GetGasFeePolicy())

	return stateMetadata.Bytes()
}

func (vmctx *vmContext) CreationSlot() iotago.SlotIndex {
	return vmctx.task.L1API().TimeProvider().SlotFromTime(vmctx.task.Timestamp)
}

func (vmctx *vmContext) BuildTransactionEssence(stateCommitment *state.L1Commitment, assertTxbuilderBalanced bool) (*iotago.Transaction, iotago.Unlocks) {
	stateMetadata := vmctx.stateMetadata(stateCommitment)
	essence, unlocks, err := vmctx.txbuilder.BuildTransactionEssence(
		stateMetadata,
		vmctx.CreationSlot(),
		vmctx.task.AllotMana,
		vmctx.task.BlockIssuerKey,
	)
	if err != nil {
		panic(vm.ErrInsufficientManaForAllotment)
	}
	if assertTxbuilderBalanced {
		vmctx.txbuilder.MustBalanced()
	}
	return essence, unlocks
}

func (vmctx *vmContext) createTxBuilderSnapshot() *vmtxbuilder.AnchorTransactionBuilder {
	return vmctx.txbuilder.Clone()
}

func (vmctx *vmContext) restoreTxBuilderSnapshot(snapshot *vmtxbuilder.AnchorTransactionBuilder) {
	vmctx.txbuilder = snapshot
}

func (vmctx *vmContext) loadNativeTokenOutput(nativeTokenID iotago.NativeTokenID) (out *iotago.BasicOutput, id iotago.OutputID) {
	return vmctx.accountsStateWriterFromChainState(vmctx.stateDraft).GetNativeTokenOutput(nativeTokenID)
}

func (vmctx *vmContext) loadFoundry(serNum uint32) (out *iotago.FoundryOutput, id iotago.OutputID) {
	return vmctx.accountsStateWriterFromChainState(vmctx.stateDraft).GetFoundryOutput(serNum, vmctx.MustChainAccountID())
}

func (vmctx *vmContext) loadNFT(nftID iotago.NFTID) (out *iotago.NFTOutput, id iotago.OutputID) {
	return vmctx.accountsStateWriterFromChainState(vmctx.stateDraft).GetNFTOutput(nftID)
}

func (vmctx *vmContext) loadTotalFungibleTokens() *isc.FungibleTokens {
	return vmctx.accountsStateWriterFromChainState(vmctx.stateDraft).GetTotalL2FungibleTokens()
}
