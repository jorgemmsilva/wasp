package vmimpl

import (
	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/packages/vm/vmtxbuilder"
)

func (vmctx *vmContext) stateMetadata(stateCommitment *state.L1Commitment) []byte {
	stateMetadata := transaction.StateMetadata{
		Version:      transaction.StateMetadataSupportedVersion,
		L1Commitment: stateCommitment,
	}

	withContractState(vmctx.stateDraft, root.Contract, func(s kv.KVStore) {
		stateMetadata.SchemaVersion = root.GetSchemaVersion(s)
	})

	withContractState(vmctx.stateDraft, governance.Contract, func(s kv.KVStore) {
		// On error, the publicURL is len(0)
		stateMetadata.PublicURL, _ = governance.GetPublicURL(s)
		stateMetadata.GasFeePolicy = lo.Must(governance.GetGasFeePolicy(s))
	})

	return stateMetadata.Bytes()
}

func (vmctx *vmContext) CreationSlot() iotago.SlotIndex {
	return vmctx.task.L1API().TimeProvider().SlotFromTime(vmctx.task.Timestamp)
}

func (vmctx *vmContext) BuildTransactionEssence(stateCommitment *state.L1Commitment, assertTxbuilderBalanced bool) (*iotago.Transaction, iotago.Unlocks) {
	stateMetadata := vmctx.stateMetadata(stateCommitment)
	essence, unlocks := vmctx.txbuilder.BuildTransactionEssence(stateMetadata, vmctx.CreationSlot())
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
	withContractState(vmctx.stateDraft, accounts.Contract, func(s kv.KVStore) {
		out, id = accounts.GetNativeTokenOutput(s, nativeTokenID, vmctx.ChainID())
	})
	return
}

func (vmctx *vmContext) loadFoundry(serNum uint32) (out *iotago.FoundryOutput, id iotago.OutputID) {
	withContractState(vmctx.stateDraft, accounts.Contract, func(s kv.KVStore) {
		out, id = accounts.GetFoundryOutput(s, serNum, vmctx.MustChainAccountID())
	})
	return
}

func (vmctx *vmContext) loadNFT(nftID iotago.NFTID) (out *iotago.NFTOutput, id iotago.OutputID) {
	withContractState(vmctx.stateDraft, accounts.Contract, func(s kv.KVStore) {
		out, id = accounts.GetNFTOutput(s, nftID)
	})
	return
}

func (vmctx *vmContext) loadTotalFungibleTokens() *isc.FungibleTokens {
	var totalAssets *isc.FungibleTokens
	withContractState(vmctx.stateDraft, accounts.Contract, func(s kv.KVStore) {
		totalAssets = accounts.GetTotalL2FungibleTokens(s)
	})
	return totalAssets
}
