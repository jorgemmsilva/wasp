package origin

import (
	"fmt"
	"time"

	"github.com/samber/lo"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/kv/subrealm"

	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/blob"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/errors"
	"github.com/iotaledger/wasp/packages/vm/core/evm"
	"github.com/iotaledger/wasp/packages/vm/core/evm/evmimpl"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/governance/governanceimpl"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/packages/vm/core/root/rootimpl"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

// L1Commitment calculates the L1 commitment for the origin state
// originDeposit must exclude the minSD for the AnchorOutput
func L1Commitment(v isc.SchemaVersion, initParams dict.Dict, originDeposit iotago.BaseToken, tokenInfo *api.InfoResBaseToken) *state.L1Commitment {
	block := InitChain(v, state.NewStoreWithUniqueWriteMutex(mapdb.NewMapDB()), initParams, originDeposit, tokenInfo)
	return block.L1Commitment()
}

const (
	ParamEVMChainID      = "a"
	ParamBlockKeepAmount = "b"
	ParamChainOwner      = "c"
	ParamWaspVersion     = "d"
)

func InitChain(v isc.SchemaVersion, store state.Store, initParams dict.Dict, originDeposit iotago.BaseToken, tokenInfo *api.InfoResBaseToken) state.Block {
	if initParams == nil {
		initParams = dict.New()
	}
	d := store.NewOriginStateDraft()
	d.Set(kv.Key(coreutil.StatePrefixBlockIndex), codec.Encode(uint32(0)))
	d.Set(kv.Key(coreutil.StatePrefixTimestamp), codec.Time.Encode(time.Unix(0, 0)))

	contractState := func(contract *coreutil.ContractInfo) kv.KVStore {
		return subrealm.New(d, kv.Key(contract.Hname().Bytes()))
	}

	evmChainID := lo.Must(codec.Uint16.Decode(initParams.Get(ParamEVMChainID), evm.DefaultChainID))
	blockKeepAmount := lo.Must(codec.Int32.Decode(initParams.Get(ParamBlockKeepAmount), governance.DefaultBlockKeepAmount))
	chainOwner := lo.Must(codec.AgentID.Decode(initParams.Get(ParamChainOwner), &isc.NilAgentID{}))

	// init the state of each core contract
	rootimpl.SetInitialState(v, contractState(root.Contract))
	blob.SetInitialState(contractState(blob.Contract))
	accounts.SetInitialState(v, contractState(accounts.Contract), originDeposit, tokenInfo)
	blocklog.SetInitialState(contractState(blocklog.Contract))
	errors.SetInitialState(contractState(errors.Contract))
	governanceimpl.SetInitialState(contractState(governance.Contract), chainOwner, blockKeepAmount)
	evmimpl.SetInitialState(contractState(evm.Contract), evmChainID)

	block := store.Commit(d)
	if err := store.SetLatest(block.TrieRoot()); err != nil {
		panic(err)
	}
	return block
}

func InitChainByAnchorOutput(chainStore state.Store, chainOutputs *isc.ChainOutputs, l1 iotago.APIProvider, tokenInfo *api.InfoResBaseToken) (state.Block, error) {
	var initParams dict.Dict
	if originMetadata := chainOutputs.AnchorOutput.FeatureSet().Metadata(); originMetadata != nil {
		var err error
		initParams, err = dict.FromBytes(originMetadata.Entries[""])
		if err != nil {
			return nil, fmt.Errorf("invalid parameters on origin AO, %w", err)
		}
	}
	anchorSD := lo.Must(chainOutputs.L1API(l1).StorageScoreStructure().MinDeposit(chainOutputs.AnchorOutput))
	commonAccountAmount := chainOutputs.AnchorOutput.Amount - anchorSD
	originAOStateMetadata, err := transaction.StateMetadataFromAnchorOutput(chainOutputs.AnchorOutput)
	originBlock := InitChain(originAOStateMetadata.SchemaVersion, chainStore, initParams, commonAccountAmount, tokenInfo)

	if err != nil {
		return nil, fmt.Errorf("invalid state metadata on origin AO: %w", err)
	}
	if originAOStateMetadata.Version != transaction.StateMetadataSupportedVersion {
		return nil, fmt.Errorf("unsupported StateMetadata Version: %v, expect %v", originAOStateMetadata.Version, transaction.StateMetadataSupportedVersion)
	}
	if !originBlock.L1Commitment().Equals(originAOStateMetadata.L1Commitment) {
		return nil, fmt.Errorf(
			"l1Commitment mismatch between originAO / originBlock: %s / %s",
			originAOStateMetadata.L1Commitment,
			originBlock.L1Commitment(),
		)
	}
	return originBlock, nil
}

func calcStateMetadata(initParams dict.Dict, commonAccountAmount iotago.BaseToken, schemaVersion isc.SchemaVersion, tokenInfo *api.InfoResBaseToken) []byte {
	s := transaction.NewStateMetadata(
		L1Commitment(schemaVersion, initParams, commonAccountAmount, tokenInfo),
		gas.DefaultFeePolicy(),
		schemaVersion,
		"",
	)
	return s.Bytes()
}

// accountOutputSD calculates the SD needed for the account output
// (which will be created on state index #1)
func accountOutputSD(api iotago.API) iotago.BaseToken {
	mockOutput := &iotago.AccountOutput{
		UnlockConditions: iotago.AccountOutputUnlockConditions{
			&iotago.AddressUnlockCondition{Address: &iotago.AnchorAddress{}},
		},
		Features: iotago.AccountOutputFeatures{
			&iotago.SenderFeature{Address: &iotago.AnchorAddress{}},
		},
	}
	return lo.Must(api.StorageScoreStructure().MinDeposit(mockOutput))
}

// NewChainOriginTransaction creates new origin transaction for the self-governed chain
// returns the transaction and newly minted chain ID
func NewChainOriginTransaction(
	keyPair cryptolib.VariantKeyPair,
	stateControllerAddress iotago.Address,
	governanceControllerAddress iotago.Address,
	deposit iotago.BaseToken,
	depositMana iotago.Mana,
	initParams dict.Dict,
	unspentOutputs iotago.OutputSet,
	creationSlot iotago.SlotIndex,
	schemaVersion isc.SchemaVersion,
	l1APIProvider iotago.APIProvider,
	tokenInfo *api.InfoResBaseToken,
) (*iotago.SignedTransaction, *isc.ChainOutputs, isc.ChainID, error) {
	walletAddr := keyPair.Address()

	if initParams == nil {
		initParams = dict.New()
	}
	if initParams.Get(ParamChainOwner) == nil {
		// default chain owner to the gov address
		initParams.Set(ParamChainOwner, isc.NewAgentID(governanceControllerAddress).Bytes())
	}

	anchorOutput := &iotago.AnchorOutput{
		Amount: deposit,
		UnlockConditions: iotago.AnchorOutputUnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{Address: stateControllerAddress},
			&iotago.GovernorAddressUnlockCondition{Address: governanceControllerAddress},
		},
		Features: iotago.AnchorOutputFeatures{
			&iotago.MetadataFeature{Entries: iotago.MetadataFeatureEntries{"": initParams.Bytes()}},
			&iotago.StateMetadataFeature{Entries: iotago.StateMetadataFeatureEntries{
				"": calcStateMetadata(initParams, deposit, schemaVersion, tokenInfo), // NOTE: Updated below.
			}},
		},
		Mana: depositMana,
	}

	l1API := l1APIProvider.APIForSlot(creationSlot)
	anchorSD := lo.Must(l1API.StorageScoreStructure().MinDeposit(anchorOutput))

	// the account output does not exist yet (will be created on the first VM run),
	// but we still must consider its SD
	accountSD := accountOutputSD(l1API)
	minAmount := anchorSD + accountSD + governance.DefaultMinBaseTokensOnCommonAccount
	if anchorOutput.Amount < minAmount {
		anchorOutput.Amount = minAmount
	}

	// update the L1 commitment to not include the minimumSD -- should not affect the SD needed
	anchorOutput.Features.Upsert(
		&iotago.StateMetadataFeature{Entries: iotago.StateMetadataFeatureEntries{
			"": calcStateMetadata(initParams, anchorOutput.Amount-anchorSD, schemaVersion, tokenInfo),
		}},
	)

	txInputs, remainder, err := transaction.ComputeInputsAndRemainder(
		walletAddr,
		unspentOutputs,
		transaction.NewAssetsWithMana(isc.FungibleTokensFromOutput(anchorOutput).ToAssets(), anchorOutput.StoredMana()),
		creationSlot,
		l1APIProvider,
	)
	if err != nil {
		return nil, nil, isc.ChainID{}, err
	}
	outputs := iotago.TxEssenceOutputs{anchorOutput}
	outputs = append(outputs, remainder...)
	tx := &iotago.Transaction{
		API: l1API,
		TransactionEssence: &iotago.TransactionEssence{
			NetworkID:    l1API.ProtocolParameters().NetworkID(),
			Inputs:       txInputs.UTXOInputs(),
			CreationSlot: creationSlot,
		},
		Outputs: outputs,
	}
	sigs, err := tx.Sign(keyPair.GetPrivateKey().AddressKeysForEd25519Address(walletAddr))
	if err != nil {
		return nil, nil, isc.ChainID{}, err
	}

	txid, err := tx.ID()
	if err != nil {
		return nil, nil, isc.ChainID{}, err
	}
	anchorOutputID := iotago.OutputIDFromTransactionIDAndIndex(txid, 0)
	chainID := isc.ChainIDFromAnchorID(iotago.AnchorIDFromOutputID(anchorOutputID))

	return &iotago.SignedTransaction{
			API:         l1API,
			Transaction: tx,
			Unlocks:     transaction.MakeSignatureAndReferenceUnlocks(len(txInputs), sigs[0]),
		},
		isc.NewChainOutputs(
			anchorOutput,
			anchorOutputID,
			nil,
			iotago.OutputID{},
		),
		chainID,
		nil
}
