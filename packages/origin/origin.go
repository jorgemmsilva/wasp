package origin

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/samber/lo"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/kv/subrealm"
	"github.com/iotaledger/wasp/packages/parameters"
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
func L1Commitment(initParams dict.Dict, originDeposit iotago.BaseToken) *state.L1Commitment {
	block := InitChain(state.NewStoreWithUniqueWriteMutex(mapdb.NewMapDB()), initParams, originDeposit)
	return block.L1Commitment()
}

const (
	ParamEVMChainID      = "a"
	ParamBlockKeepAmount = "b"
	ParamChainOwner      = "c"
	ParamWaspVersion     = "d"
)

func InitChain(store state.Store, initParams dict.Dict, originDeposit iotago.BaseToken) state.Block {
	if initParams == nil {
		initParams = dict.New()
	}
	d := store.NewOriginStateDraft()
	d.Set(kv.Key(coreutil.StatePrefixBlockIndex), codec.Encode(uint32(0)))
	d.Set(kv.Key(coreutil.StatePrefixTimestamp), codec.EncodeTime(time.Unix(0, 0)))

	contractState := func(contract *coreutil.ContractInfo) kv.KVStore {
		return subrealm.New(d, kv.Key(contract.Hname().Bytes()))
	}

	evmChainID := codec.MustDecodeUint16(initParams.Get(ParamEVMChainID), evm.DefaultChainID)
	blockKeepAmount := codec.MustDecodeInt32(initParams.Get(ParamBlockKeepAmount), governance.DefaultBlockKeepAmount)
	chainOwner := codec.MustDecodeAgentID(initParams.Get(ParamChainOwner), &isc.NilAgentID{})

	// init the state of each core contract
	rootimpl.SetInitialState(contractState(root.Contract))
	blob.SetInitialState(contractState(blob.Contract))
	accounts.SetInitialState(contractState(accounts.Contract), originDeposit)
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

func InitChainByAnchorOutput(chainStore state.Store, chainOutputs *isc.ChainOutputs) (state.Block, error) {
	var initParams dict.Dict
	if originMetadata := chainOutputs.AnchorOutput.FeatureSet().Metadata(); originMetadata != nil {
		var err error
		initParams, err = dict.FromBytes(originMetadata.Entries[""])
		if err != nil {
			return nil, fmt.Errorf("invalid parameters on origin AO, %w", err)
		}
	}
	api := parameters.L1Provider().APIForSlot(chainOutputs.AnchorOutputID.CreationSlot())
	anchorSD := lo.Must(api.StorageScoreStructure().MinDeposit(chainOutputs.AnchorOutput))
	commonAccountAmount := chainOutputs.AnchorOutput.Amount - anchorSD
	originBlock := InitChain(chainStore, initParams, commonAccountAmount)

	originAOStateMetadata, err := transaction.StateMetadataFromAnchorOutput(chainOutputs.AnchorOutput)
	if err != nil {
		return nil, fmt.Errorf("invalid state metadata on origin AO: %w", err)
	}
	if originAOStateMetadata.Version != transaction.StateMetadataSupportedVersion {
		return nil, fmt.Errorf("unsupported StateMetadata Version: %v, expect %v", originAOStateMetadata.Version, transaction.StateMetadataSupportedVersion)
	}
	if !originBlock.L1Commitment().Equals(originAOStateMetadata.L1Commitment) {
		l1paramsJSON, err := json.Marshal(parameters.L1())
		if err != nil {
			l1paramsJSON = []byte(fmt.Sprintf("unable to marshalJson l1params: %s", err.Error()))
		}
		return nil, fmt.Errorf(
			"l1Commitment mismatch between originAO / originBlock: %s / %s, L1params: %s",
			originAOStateMetadata.L1Commitment,
			originBlock.L1Commitment(),
			string(l1paramsJSON),
		)
	}
	return originBlock, nil
}

func calcStateMetadata(initParams dict.Dict, commonAccountAmount iotago.BaseToken, schemaVersion uint32) []byte {
	s := transaction.NewStateMetadata(
		L1Commitment(initParams, commonAccountAmount),
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
	keyPair *cryptolib.KeyPair,
	stateControllerAddress iotago.Address,
	governanceControllerAddress iotago.Address,
	deposit iotago.BaseToken,
	depositMana iotago.Mana,
	initParams dict.Dict,
	unspentOutputs iotago.OutputSet,
	creationSlot iotago.SlotIndex,
	schemaVersion uint32,
) (*iotago.SignedTransaction, *isc.ChainOutputs, isc.ChainID, error) {
	walletAddr := keyPair.GetPublicKey().AsEd25519Address()

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
			// SenderFeature included so that SD calculation keeps stable.
			// SenderFeature will be set to AnchorAddress in the first VM run.
			&iotago.SenderFeature{Address: walletAddr},
			&iotago.MetadataFeature{Entries: iotago.MetadataFeatureEntries{"": initParams.Bytes()}},
			&iotago.StateMetadataFeature{Entries: iotago.StateMetadataFeatureEntries{
				"": calcStateMetadata(initParams, deposit, schemaVersion), // NOTE: Updated below.
			}},
		},
		Mana: depositMana,
	}
	anchorSD := lo.Must(parameters.Storage().MinDeposit(anchorOutput))

	accountSD := accountOutputSD(parameters.L1API())
	minAmount := anchorSD + accountSD + governance.DefaultMinBaseTokensOnCommonAccount
	if anchorOutput.Amount < minAmount {
		anchorOutput.Amount = minAmount
	}

	// update the L1 commitment to not include the minimumSD -- should not affect the SD needed
	anchorOutput.Features.Upsert(
		&iotago.StateMetadataFeature{Entries: iotago.StateMetadataFeatureEntries{
			"": calcStateMetadata(initParams, anchorOutput.Amount-anchorSD, schemaVersion),
		}},
	)

	txInputs, remainder, err := transaction.ComputeInputsAndRemainder(
		walletAddr,
		unspentOutputs,
		transaction.NewAssetsWithMana(isc.FungibleTokensFromOutput(anchorOutput).ToAssets(), anchorOutput.StoredMana()),
		creationSlot,
	)
	if err != nil {
		return nil, nil, isc.ChainID{}, err
	}
	outputs := iotago.TxEssenceOutputs{anchorOutput}
	outputs = append(outputs, remainder...)
	tx := &iotago.Transaction{
		API: parameters.L1API(),
		TransactionEssence: &iotago.TransactionEssence{
			NetworkID:    parameters.L1().Protocol.NetworkID(),
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
			API:         parameters.L1API(),
			Transaction: tx,
			Unlocks:     transaction.MakeSignatureAndReferenceUnlocks(len(txInputs), sigs[0]),
		},
		isc.NewChainOuptuts(
			anchorOutput,
			anchorOutputID,
			nil,
			iotago.OutputID{},
		),
		chainID,
		nil
}
