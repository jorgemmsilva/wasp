package vmtxbuilder

import (
	"fmt"
	"math/big"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/vm"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/vmexceptions"
)

type AccountsContractRead struct {
	// nativeTokenOutputLoaderFunc loads stored output from the state
	// Should return nil if does not exist
	NativeTokenOutput func(iotago.NativeTokenID) (*iotago.BasicOutput, iotago.OutputID)

	// foundryLoaderFunc returns foundry output and id by its serial number
	// Should return nil if foundry does not exist
	FoundryOutput func(uint32) (*iotago.FoundryOutput, iotago.OutputID)

	// NFTOutput returns the stored NFT output from the state
	// Should return nil if NFT is not accounted for
	NFTOutput func(id iotago.NFTID) (*iotago.NFTOutput, iotago.OutputID)

	// TotalFungibleTokens returns the total base tokens and native tokens accounted by the chain
	TotalFungibleTokens func() *isc.FungibleTokens
}

// AnchorTransactionBuilder represents structure which handles all the data needed to eventually
// build an essence of the anchor transaction
type AnchorTransactionBuilder struct {
	// anchor output of the chain
	inputs *isc.ChainOutputs

	// result new AO of the chain, filled by "BuildTransactionEssence"
	resultAnchorOutput  *iotago.AnchorOutput
	resultAccountOutput *iotago.AccountOutput

	// already consumed outputs, specified by entire Request. It is needed for checking validity
	consumed []isc.OnLedgerRequest

	// view the accounts contract state
	accountsView AccountsContractRead

	// balances of native tokens loaded during the batch run
	balanceNativeTokens map[iotago.NativeTokenID]*nativeTokenBalance
	// all nfts loaded during the batch run
	nftsIncluded map[iotago.NFTID]*nftIncluded
	// all nfts minted
	nftsMinted iotago.TxEssenceOutputs
	// invoked foundries. Foundry serial number is used as a key
	invokedFoundries map[uint32]*foundryInvoked
	// requests posted by smart contracts
	postedOutputs iotago.TxEssenceOutputs
}

// NewAnchorTransactionBuilder creates new AnchorTransactionBuilder object
func NewAnchorTransactionBuilder(
	inputs *isc.ChainOutputs,
	accounts AccountsContractRead,
) *AnchorTransactionBuilder {
	return &AnchorTransactionBuilder{
		inputs:              inputs,
		accountsView:        accounts,
		consumed:            make([]isc.OnLedgerRequest, 0, iotago.MaxInputsCount-1),
		balanceNativeTokens: make(map[iotago.NativeTokenID]*nativeTokenBalance),
		postedOutputs:       make(iotago.TxEssenceOutputs, 0, iotago.MaxOutputsCount-1),
		invokedFoundries:    make(map[uint32]*foundryInvoked),
		nftsIncluded:        make(map[iotago.NFTID]*nftIncluded),
		nftsMinted:          make(iotago.TxEssenceOutputs, 0),
	}
}

// Clone clones the AnchorTransactionBuilder object. Used to snapshot/recover
func (txb *AnchorTransactionBuilder) Clone() *AnchorTransactionBuilder {
	return &AnchorTransactionBuilder{
		inputs:              txb.inputs,
		accountsView:        txb.accountsView,
		consumed:            util.CloneSlice(txb.consumed),
		balanceNativeTokens: util.CloneMap(txb.balanceNativeTokens),
		postedOutputs: lo.Map(txb.postedOutputs, func(o iotago.TxEssenceOutput, _ int) iotago.TxEssenceOutput {
			return o.Clone()
		}),
		invokedFoundries: util.CloneMap(txb.invokedFoundries),
		nftsIncluded:     util.CloneMap(txb.nftsIncluded),
		nftsMinted: lo.Map(txb.nftsMinted, func(o iotago.TxEssenceOutput, _ int) iotago.TxEssenceOutput {
			return o.Clone()
		}),
	}
}

// splitAssetsIntoInternalOutputs splits the native Tokens/NFT from a given (request) output.
// returns the resulting outputs and the list of new outputs
// (some of the native tokens might already have an accounting output owned by the chain, so we don't need new outputs for those)
func (txb *AnchorTransactionBuilder) splitAssetsIntoInternalOutputs(req isc.OnLedgerRequest) iotago.BaseToken {
	requiredSD := iotago.BaseToken(0)
	for id, amount := range req.Assets().NativeTokens {
		// ensure this NT is in the txbuilder, update it
		nt := txb.ensureNativeTokenBalance(id)
		sdBefore := nt.accountingOutput.Amount
		if util.IsZeroBigInt(nt.getOutValue()) {
			sdBefore = 0 // accounting output was zero'ed this block, meaning the existing SD was released
		}
		nt.add(amount)
		nt.updateMinSD()
		sdAfter := nt.accountingOutput.Amount
		// user pays for the difference (in case SD has increased, will be the full SD cost if the output is new)
		requiredSD += sdAfter - sdBefore
	}

	if req.NFT() != nil {
		// create new output
		nftIncl := txb.internalNFTOutputFromRequest(req.Output().(*iotago.NFTOutput), req.OutputID())
		requiredSD += nftIncl.resultingOutput.Amount
	}

	txb.consumed = append(txb.consumed, req)
	return requiredSD
}

func (txb *AnchorTransactionBuilder) assertLimits() {
	if txb.InputsAreFull() {
		panic(vmexceptions.ErrInputLimitExceeded)
	}
	if txb.outputsAreFull() {
		panic(vmexceptions.ErrOutputLimitExceeded)
	}
}

// Consume adds an input to the transaction.
// It panics if transaction cannot hold that many inputs
// All explicitly consumed inputs will hold fixed index in the transaction
// It updates total assets held by the chain. So it may panic due to exceed output counts
// Returns  the amount of baseTokens needed to cover SD costs for the NTs/NFT contained by the request output
func (txb *AnchorTransactionBuilder) Consume(req isc.OnLedgerRequest) iotago.BaseToken {
	defer txb.assertLimits()
	// deduct the minSD for all the outputs that need to be created
	requiredSD := txb.splitAssetsIntoInternalOutputs(req)
	return requiredSD
}

// ConsumeUnprocessable adds an unprocessable request to the txbuilder,
// consumes the original request and cretes a new output keeping assets intact
// return the position of the resulting output in `txb.postedOutputs`
func (txb *AnchorTransactionBuilder) ConsumeUnprocessable(req isc.OnLedgerRequest) int {
	defer txb.assertLimits()
	txb.consumed = append(txb.consumed, req)
	txb.postedOutputs = append(txb.postedOutputs, retryOutputFromOnLedgerRequest(req, txb.inputs.AnchorOutput.AnchorID))
	return len(txb.postedOutputs) - 1
}

// AddOutput adds an information about posted request. It will produce output
// Return adjustment needed for the L2 ledger (adjustment on base tokens related to storage deposit)
func (txb *AnchorTransactionBuilder) AddOutput(o iotago.Output) int64 {
	defer txb.assertLimits()

	storageDeposit, err := parameters.Storage().MinDeposit(o)
	if err != nil {
		panic(err)
	}
	if o.BaseTokenAmount() < storageDeposit {
		panic(fmt.Errorf("%v: available %d < required %d base tokens",
			transaction.ErrNotEnoughBaseTokensForStorageDeposit, o.BaseTokenAmount(), storageDeposit))
	}
	fts := isc.FungibleTokensFromOutput(o)

	sdAdjustment := int64(0)
	for id, amount := range fts.NativeTokens {
		sdAdjustment += txb.addNativeTokenBalanceDelta(id, new(big.Int).Neg(amount))
	}
	if nftout, ok := o.(*iotago.NFTOutput); ok {
		sdAdjustment += txb.sendNFT(nftout)
	}
	txb.postedOutputs = append(txb.postedOutputs, o)
	return sdAdjustment
}

// InputsAreFull returns if transaction cannot hold more inputs
func (txb *AnchorTransactionBuilder) InputsAreFull() bool {
	return txb.numInputs() >= iotago.MaxInputsCount
}

// BuildTransactionEssence builds transaction essence from tx builder data
func (txb *AnchorTransactionBuilder) BuildTransactionEssence(stateMetadata []byte, creationSlot iotago.SlotIndex) (*iotago.Transaction, iotago.Unlocks) {
	inputs, inputIDs, unlocks := txb.buildInputs()
	return &iotago.Transaction{
		API: parameters.L1API(),
		TransactionEssence: &iotago.TransactionEssence{
			CreationSlot: creationSlot,
			NetworkID:    parameters.Protocol().NetworkID(),
			Inputs:       inputIDs.UTXOInputs(),
			Capabilities: iotago.TransactionCapabilitiesBitMaskWithCapabilities(
				iotago.WithTransactionCanDestroyFoundryOutputs(true),
			),
		},
		Outputs: txb.buildOutputs(stateMetadata, creationSlot, inputs),
	}, unlocks
}

// buildInputs generates a deterministic list of inputs for the transaction essence
// - index 0 is always alias output
// - then goes consumed external BasicOutput UTXOs, the requests, in the order of processing
// - then goes inputs of native token UTXOs, sorted by token id
// - then goes inputs of foundries sorted by serial number
func (txb *AnchorTransactionBuilder) buildInputs() (iotago.OutputSet, iotago.OutputIDs, iotago.Unlocks) {
	outputIDs := make(iotago.OutputIDs, 0, len(txb.consumed)+len(txb.balanceNativeTokens)+len(txb.invokedFoundries))
	inputs := make(iotago.OutputSet)
	unlocks := make(iotago.Unlocks, 0, len(outputIDs))

	// anchor output
	outputIDs = append(outputIDs, txb.inputs.AnchorOutputID)
	inputs[txb.inputs.AnchorOutputID] = txb.inputs.AnchorOutput
	unlocks = append(unlocks, &iotago.SignatureUnlock{}) // to be filled with the actual signature

	// account output
	if id, out, ok := txb.inputs.AccountOutput(); ok {
		outputIDs = append(outputIDs, id)
		inputs[id] = out
		unlocks = append(unlocks, &iotago.AnchorUnlock{Reference: 0})
	}

	// consumed on-ledger requests
	for i := range txb.consumed {
		req := txb.consumed[i]
		outputID := req.OutputID()
		output := req.Output()
		if retrReq, ok := req.(*isc.RetryOnLedgerRequest); ok {
			outputID = retrReq.RetryOutputID()
			output = retryOutputFromOnLedgerRequest(req, txb.inputs.AnchorOutput.AnchorID)
		}
		outputIDs = append(outputIDs, outputID)
		inputs[outputID] = output
		unlocks = append(unlocks, &iotago.AnchorUnlock{Reference: 0})
	}

	// internal native token outputs
	for _, nativeTokenBalance := range txb.nativeTokenOutputsSorted() {
		if nativeTokenBalance.requiresExistingAccountingUTXOAsInput() {
			outputID := nativeTokenBalance.accountingInputID
			outputIDs = append(outputIDs, outputID)
			inputs[outputID] = nativeTokenBalance.accountingInput
			unlocks = append(unlocks, &iotago.AnchorUnlock{Reference: 0})
		}
	}

	// foundries
	for _, foundry := range txb.foundriesSorted() {
		if foundry.requiresExistingAccountingUTXOAsInput() {
			outputID := foundry.accountingInputID
			outputIDs = append(outputIDs, outputID)
			inputs[outputID] = foundry.accountingInput
			unlocks = append(unlocks, &iotago.AccountUnlock{Reference: 1})
		}
	}

	// nfts
	for _, nft := range txb.nftsSorted() {
		if !isc.IsEmptyOutputID(nft.accountingInputID) {
			outputID := nft.accountingInputID
			outputIDs = append(outputIDs, outputID)
			inputs[outputID] = nft.accountingInput
			unlocks = append(unlocks, &iotago.AnchorUnlock{Reference: 0})
		}
	}

	if len(outputIDs) != txb.numInputs() {
		panic(fmt.Sprintf("AnchorTransactionBuilder.inputs: internal inconsistency. expected: %d actual:%d", len(outputIDs), txb.numInputs()))
	}
	return inputs, outputIDs, unlocks
}

func (txb *AnchorTransactionBuilder) AccountID() iotago.AccountID {
	id, out, ok := txb.inputs.AccountOutput()
	if !ok {
		panic("AccountID unknown")
	}
	return util.AccountIDFromAccountOutput(out, id)
}

func (txb *AnchorTransactionBuilder) getSDInChainOutputs() iotago.BaseToken {
	api := parameters.L1Provider().APIForSlot(txb.inputs.AnchorOutputID.CreationSlot())
	ret := lo.Must(api.StorageScoreStructure().MinDeposit(txb.inputs.AnchorOutput))
	if _, out, ok := txb.inputs.AccountOutput(); ok {
		ret += out.Amount
	}
	return ret
}

func (txb *AnchorTransactionBuilder) ChangeInSD(
	stateMetadata []byte,
	creationSlot iotago.SlotIndex,
) (iotago.BaseToken, iotago.BaseToken, int64) {
	mockAnchor, mockAccount := txb.CreateAnchorAndAccountOutputs(
		stateMetadata,
		creationSlot,
		txb.inputs.AnchorOutput.Mana,
	)
	newSD := lo.Must(parameters.Storage().MinDeposit(mockAnchor)) + mockAccount.Amount
	oldSD := txb.getSDInChainOutputs()
	return oldSD, newSD, int64(oldSD) - int64(newSD)
}

func (txb *AnchorTransactionBuilder) CreateAnchorAndAccountOutputs(
	stateMetadata []byte,
	creationSlot iotago.SlotIndex,
	totalManaInInputs iotago.Mana,
) (*iotago.AnchorOutput, *iotago.AccountOutput) {
	anchorID := txb.inputs.AnchorOutput.AnchorID
	if anchorID.Empty() {
		anchorID = iotago.AnchorIDFromOutputID(txb.inputs.AnchorOutputID)
	}
	anchorOutput := &iotago.AnchorOutput{
		Amount:     0,
		AnchorID:   anchorID,
		StateIndex: txb.inputs.AnchorOutput.StateIndex + 1,
		UnlockConditions: iotago.AnchorOutputUnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{Address: txb.inputs.AnchorOutput.StateController()},
			&iotago.GovernorAddressUnlockCondition{Address: txb.inputs.AnchorOutput.GovernorAddress()},
		},
		Features: iotago.AnchorOutputFeatures{
			&iotago.StateMetadataFeature{Entries: iotago.StateMetadataFeatureEntries{"": stateMetadata}},
		},
		Mana: totalManaInInputs,
	}
	if metadata := txb.inputs.AnchorOutput.FeatureSet().Metadata(); metadata != nil {
		anchorOutput.Features.Upsert(&iotago.MetadataFeature{Entries: metadata.Entries})
		anchorOutput.Features.Sort()
	}
	anchorOutput.Amount = txb.accountsView.TotalFungibleTokens().BaseTokens + lo.Must(parameters.Storage().MinDeposit(anchorOutput))

	accountOutput := &iotago.AccountOutput{
		FoundryCounter: txb.nextFoundryCounter(),
		UnlockConditions: iotago.AccountOutputUnlockConditions{
			&iotago.AddressUnlockCondition{Address: anchorID.ToAddress()},
		},
		Features: iotago.AccountOutputFeatures{
			&iotago.SenderFeature{Address: anchorID.ToAddress()},
		},
	}
	if id, out, ok := txb.inputs.AccountOutput(); ok {
		accountOutput.AccountID = out.AccountID
		if accountOutput.AccountID.Empty() {
			accountOutput.AccountID = iotago.AccountIDFromOutputID(id)
		}
	}
	accountOutput.Amount = lo.Must(parameters.Storage().MinDeposit(accountOutput))

	return anchorOutput, accountOutput
}

// outputs generates outputs for the transaction essence
// IMPORTANT: the order that assets are added here must not change, otherwise vmctx.saveInternalUTXOs will be broken.
// 0. Anchor Output
// 1. NativeTokens
// 2. Foundries
// 3. received NFTs
// 4. minted NFTs
// 5. other outputs (posted from requests)
func (txb *AnchorTransactionBuilder) buildOutputs(
	stateMetadata []byte,
	creationSlot iotago.SlotIndex,
	inputs iotago.OutputSet,
) iotago.TxEssenceOutputs {
	ret := make(iotago.TxEssenceOutputs, 0, 1+len(txb.balanceNativeTokens)+len(txb.postedOutputs))

	totalMana := lo.Must(vm.TotalManaIn(
		parameters.L1API().ManaDecayProvider(),
		parameters.Storage(),
		creationSlot,
		vm.InputSet(inputs),
		vm.RewardsInputSet{},
	))

	txb.resultAnchorOutput, txb.resultAccountOutput = txb.CreateAnchorAndAccountOutputs(stateMetadata, creationSlot, totalMana)
	ret = append(ret, txb.resultAnchorOutput, txb.resultAccountOutput)

	// creating outputs for updated internal accounts
	nativeTokensToBeUpdated, _ := txb.NativeTokenRecordsToBeUpdated()
	for _, id := range nativeTokensToBeUpdated {
		// create one output for each token ID of internal account
		ret = append(ret, txb.balanceNativeTokens[id].accountingOutput)
	}
	// creating outputs for updated foundries
	foundriesToBeUpdated, _ := txb.FoundriesToBeUpdated()
	for _, sn := range foundriesToBeUpdated {
		ret = append(ret, txb.invokedFoundries[sn].accountingOutput)
	}
	// creating outputs for received NFTs
	nftOuts := txb.NFTOutputs()
	for _, nftOut := range nftOuts {
		ret = append(ret, nftOut)
	}
	// creating outputs for minted NFTs
	ret = append(ret, txb.nftsMinted...)

	// creating outputs for posted on-ledger requests
	ret = append(ret, txb.postedOutputs...)
	return ret
}

// numInputs number of inputs in the future transaction
func (txb *AnchorTransactionBuilder) numInputs() int {
	ret := len(txb.consumed) + 1 // + 1 for anchor UTXO
	if _, _, ok := txb.inputs.AccountOutput(); ok {
		ret += 1
	}
	for _, v := range txb.balanceNativeTokens {
		if v.requiresExistingAccountingUTXOAsInput() {
			ret++
		}
	}
	for _, f := range txb.invokedFoundries {
		if f.requiresExistingAccountingUTXOAsInput() {
			ret++
		}
	}
	for _, nft := range txb.nftsIncluded {
		if !isc.IsEmptyOutputID(nft.accountingInputID) {
			ret++
		}
	}
	return ret
}

// numOutputs in the transaction
func (txb *AnchorTransactionBuilder) numOutputs() int {
	ret := 2 // for chain output + account output
	for _, v := range txb.balanceNativeTokens {
		if v.producesAccountingOutput() {
			ret++
		}
	}
	ret += len(txb.postedOutputs)
	for _, f := range txb.invokedFoundries {
		if f.producesAccountingOutput() {
			ret++
		}
	}
	ret += len(txb.nftsMinted)
	return ret
}

// outputsAreFull return if transaction cannot bear more outputs
func (txb *AnchorTransactionBuilder) outputsAreFull() bool {
	return txb.numOutputs() >= iotago.MaxOutputsCount
}

func retryOutputFromOnLedgerRequest(req isc.OnLedgerRequest, chainAnchorID iotago.AnchorID) iotago.Output {
	out := req.Output().Clone()

	features := []iotago.Feature{
		&iotago.SenderFeature{
			Address: chainAnchorID.ToAddress(), // must have the chain as the sender, so its recognized as an internalUTXO
		},
	}
	ntFeature := out.FeatureSet().NativeToken()
	if ntFeature != nil {
		features = append(features, ntFeature.Clone()) // keep NT feature, if it exists
	}

	unlock := &iotago.AddressUnlockCondition{
		Address: chainAnchorID.ToAddress(),
	}

	// cleanup features and unlock conditions
	switch o := out.(type) {
	case *iotago.BasicOutput:
		o.Features = features
		o.UnlockConditions = iotago.BasicOutputUnlockConditions{unlock}
	case *iotago.NFTOutput:
		o.Features = features
		o.UnlockConditions = iotago.NFTOutputUnlockConditions{unlock}
	case *iotago.AnchorOutput:
		o.Features = features
		o.UnlockConditions = iotago.AnchorOutputUnlockConditions{unlock}
	default:
		panic("unexpected output type")
	}
	return out
}

func (txb *AnchorTransactionBuilder) chainAddress() iotago.Address {
	return txb.inputs.AnchorOutput.AnchorID.ToAddress()
}
