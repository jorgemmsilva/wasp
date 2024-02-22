package vmimpl

import (
	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm"
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

	stateMetadata.SchemaVersion = root.NewStateAccess(vmctx.stateDraft).SchemaVersion()

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
	vmctx.withAccountsState(vmctx.stateDraft, func(s *accounts.StateWriter) {
		out, id = s.GetNativeTokenOutput(nativeTokenID)
	})
	return
}

func (vmctx *vmContext) loadFoundry(serNum uint32) (out *iotago.FoundryOutput, id iotago.OutputID) {
	vmctx.withAccountsState(vmctx.stateDraft, func(s *accounts.StateWriter) {
		out, id = s.GetFoundryOutput(serNum, vmctx.MustChainAccountID())
	})
	return
}

func (vmctx *vmContext) loadNFT(nftID iotago.NFTID) (out *iotago.NFTOutput, id iotago.OutputID) {
	vmctx.withAccountsState(vmctx.stateDraft, func(s *accounts.StateWriter) {
		out, id = s.GetNFTOutput(nftID)
	})
	return
}

func (vmctx *vmContext) loadTotalFungibleTokens() *isc.FungibleTokens {
	var ret *isc.FungibleTokens
	vmctx.withAccountsState(vmctx.stateDraft, func(s *accounts.StateWriter) {
		ret = s.GetTotalL2FungibleTokens()
	})
	return ret
}
