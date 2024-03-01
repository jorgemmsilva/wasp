package origin

import (
	"fmt"
	"math"
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
	"github.com/iotaledger/wasp/packages/util"

	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/blob"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/errors"
	"github.com/iotaledger/wasp/packages/vm/core/evm"
	"github.com/iotaledger/wasp/packages/vm/core/evm/evmimpl"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

// L1Commitment calculates the L1 commitment for the origin state
// originDeposit must exclude the minSD for the AnchorOutput
func L1Commitment(
	v isc.SchemaVersion,
	initParams dict.Dict,
	originDeposit iotago.BaseToken,
	tokenInfo *api.InfoResBaseToken,
	l1API iotago.API,
) *state.L1Commitment {
	block := InitChain(
		v,
		state.NewStoreWithUniqueWriteMutex(mapdb.NewMapDB()),
		initParams,
		originDeposit,
		tokenInfo,
		l1API,
	)
	return block.L1Commitment()
}

const (
	ParamEVMChainID      = "a"
	ParamBlockKeepAmount = "b"
	ParamChainOwner      = "c"
	ParamWaspVersion     = "d"
)

func InitChain(
	v isc.SchemaVersion,
	store state.Store,
	initParams dict.Dict,
	originDeposit iotago.BaseToken,
	tokenInfo *api.InfoResBaseToken,
	l1API iotago.API,
) state.Block {
	if initParams == nil {
		initParams = dict.New()
	}
	d := store.NewOriginStateDraft()
	d.Set(kv.Key(coreutil.StatePrefixBlockIndex), codec.Encode(uint32(0)))
	d.Set(kv.Key(coreutil.StatePrefixTimestamp), codec.Time.Encode(time.Unix(0, 0)))

	evmChainID := lo.Must(codec.Uint16.Decode(initParams.Get(ParamEVMChainID), evm.DefaultChainID))
	blockKeepAmount := lo.Must(codec.Int32.Decode(initParams.Get(ParamBlockKeepAmount), governance.DefaultBlockKeepAmount))
	chainOwner := lo.Must(codec.AgentID.Decode(initParams.Get(ParamChainOwner), &isc.NilAgentID{}))

	// init the state of each core contract
	root.NewStateWriter(root.Contract.StateSubrealm(d)).SetInitialState(v, []*coreutil.ContractInfo{
		root.Contract,
		blob.Contract,
		accounts.Contract,
		blocklog.Contract,
		errors.Contract,
		governance.Contract,
		evm.Contract,
	})
	blob.NewStateWriter(blob.Contract.StateSubrealm(d)).SetInitialState()
	accounts.NewStateWriter(
		accounts.NewStateContext(v, isc.ChainID{}, tokenInfo, l1API),
		accounts.Contract.StateSubrealm(d),
	).
		SetInitialState(originDeposit)
	blocklog.NewStateWriter(blocklog.Contract.StateSubrealm(d)).SetInitialState()
	errors.NewStateWriter(errors.Contract.StateSubrealm(d)).SetInitialState()
	governance.NewStateWriter(governance.Contract.StateSubrealm(d)).SetInitialState(chainOwner, blockKeepAmount)
	evmimpl.SetInitialState(evm.Contract.StateSubrealm(d), evmChainID)

	block := store.Commit(d)
	if err := store.SetLatest(block.TrieRoot()); err != nil {
		panic(err)
	}
	return block
}

func InitChainByAnchorOutput(
	chainStore state.Store,
	chainOutputs *isc.ChainOutputs,
	l1 iotago.APIProvider,
	tokenInfo *api.InfoResBaseToken,
) (state.Block, error) {
	var initParams dict.Dict
	if originMetadata := chainOutputs.AnchorOutput.FeatureSet().Metadata(); originMetadata != nil {
		var err error
		initParams, err = dict.FromBytes(originMetadata.Entries[""])
		if err != nil {
			return nil, fmt.Errorf("invalid parameters on origin AO, %w", err)
		}
	}
	l1API := chainOutputs.L1API(l1)
	anchorSD := lo.Must(l1API.StorageScoreStructure().MinDeposit(chainOutputs.AnchorOutput))
	commonAccountAmount := chainOutputs.AnchorOutput.Amount - anchorSD
	originAOStateMetadata, err := transaction.StateMetadataFromAnchorOutput(chainOutputs.AnchorOutput)
	originBlock := InitChain(
		originAOStateMetadata.SchemaVersion,
		chainStore, initParams,
		commonAccountAmount,
		tokenInfo,
		l1API,
	)

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

func calcStateMetadata(
	initParams dict.Dict,
	commonAccountAmount iotago.BaseToken,
	schemaVersion isc.SchemaVersion,
	tokenInfo *api.InfoResBaseToken,
	l1API iotago.API,
) []byte {
	s := transaction.NewStateMetadata(
		L1Commitment(schemaVersion, initParams, commonAccountAmount, tokenInfo, l1API),
		gas.DefaultFeePolicy(),
		schemaVersion,
		"",
	)
	return s.Bytes()
}

// NewChainOriginTransaction creates new origin transaction for the self-governed chain
// returns the transaction and newly minted chain ID
// deposit - these tokens will cover minSD of the anchorOutput + minSD of the AccountOutput + governance.DefaultMinBaseTokensOnCommonAccount. If not enough, a higher value will be used
//
//nolint:funlen
func NewChainOriginTransaction(
	keyPair cryptolib.VariantKeyPair,
	stateControllerPubKey *cryptolib.PublicKey,
	governanceControllerAddress iotago.Address,
	deposit iotago.BaseToken,
	initParams dict.Dict,
	unspentOutputs iotago.OutputSet,
	creationSlot iotago.SlotIndex,
	schemaVersion isc.SchemaVersion,
	l1APIProvider iotago.APIProvider,
	blockIssuance *api.IssuanceBlockHeaderResponse,
	tokenInfo *api.InfoResBaseToken,
) (*iotago.Block, *isc.ChainOutputs, iotago.AccountID, isc.ChainID, error) {
	walletAddr := keyPair.Address()

	if initParams == nil {
		initParams = dict.New()
	}
	if initParams.Get(ParamChainOwner) == nil {
		// default chain owner to the gov address
		initParams.Set(ParamChainOwner, isc.NewAgentID(governanceControllerAddress).Bytes())
	}

	l1API := l1APIProvider.APIForSlot(creationSlot)

	//
	// create an account output for the committee (with minSD)
	accountOutput := &iotago.AccountOutput{
		Amount:         0,
		AccountID:      iotago.EmptyAccountID,
		FoundryCounter: 0,
		UnlockConditions: []iotago.AccountOutputUnlockCondition{
			&iotago.AddressUnlockCondition{
				Address: stateControllerPubKey.AsEd25519Address(),
			},
		},
		Features: []iotago.AccountOutputFeature{
			&iotago.BlockIssuerFeature{
				ExpirySlot: math.MaxUint32,
				BlockIssuerKeys: []iotago.BlockIssuerKey{
					iotago.Ed25519PublicKeyHashBlockIssuerKeyFromPublicKey(stateControllerPubKey.AsHiveEd25519PubKey()),
				},
			},
		},
	}
	accountOutput.Amount = lo.Must(l1API.StorageScoreStructure().MinDeposit(accountOutput))

	//
	// add the anchor output for the chain
	anchorOutput := &iotago.AnchorOutput{
		Amount: 0,
		UnlockConditions: iotago.AnchorOutputUnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{Address: stateControllerPubKey.AsEd25519Address()},
			&iotago.GovernorAddressUnlockCondition{Address: governanceControllerAddress},
		},
		Features: iotago.AnchorOutputFeatures{
			&iotago.MetadataFeature{Entries: iotago.MetadataFeatureEntries{"": initParams.Bytes()}},
			&iotago.StateMetadataFeature{Entries: iotago.StateMetadataFeatureEntries{
				"": calcStateMetadata(initParams, deposit, schemaVersion, tokenInfo, l1API), // NOTE: Updated below.
			}},
		},
		Mana: 0,
	}

	anchorSD := lo.Must(l1API.StorageScoreStructure().MinDeposit(anchorOutput))

	// adjust base tokens available for the anchorOuput
	minAmount := anchorSD + governance.DefaultMinBaseTokensOnCommonAccount
	if deposit < minAmount+accountOutput.Amount {
		anchorOutput.Amount = minAmount
	} else {
		anchorOutput.Amount = deposit - accountOutput.Amount
	}

	// update the L1 commitment to not include the minimumSD -- should not affect the SD needed
	anchorOutput.Features.Upsert(
		&iotago.StateMetadataFeature{Entries: iotago.StateMetadataFeatureEntries{
			"": calcStateMetadata(initParams, anchorOutput.Amount-anchorSD, schemaVersion, tokenInfo, l1API),
		}},
	)

	txInputs, remainder, blockIssuerAccountID, err := transaction.ComputeInputsAndRemainder(
		walletAddr,
		unspentOutputs,
		transaction.NewAssetsWithMana(isc.NewAssetsBaseTokens(anchorOutput.Amount+accountOutput.Amount), 0),
		creationSlot,
		l1APIProvider,
	)
	if err != nil {
		return nil, nil, iotago.EmptyAccountID, isc.ChainID{}, err
	}
	outputs := []iotago.Output{anchorOutput, accountOutput} // anchor is output index 0
	outputs = append(outputs, remainder...)

	block, err := transaction.FinalizeTxAndBuildBlock(
		l1API,
		transaction.TxBuilderFromInputsAndOutputs(l1API, txInputs, outputs, keyPair),
		blockIssuance,
		len(outputs)-1, // store the deployers mana in the last output
		blockIssuerAccountID,
		keyPair,
	)
	if err != nil {
		return nil, nil, iotago.EmptyAccountID, isc.ChainID{}, err
	}

	txid, err := util.TxFromBlock(block).Transaction.ID()
	if err != nil {
		return nil, nil, iotago.EmptyAccountID, isc.ChainID{}, err
	}
	anchorOutputID := iotago.OutputIDFromTransactionIDAndIndex(txid, 0) // we know the anchor is on output index 0
	chainID := isc.ChainIDFromAnchorID(iotago.AnchorIDFromOutputID(anchorOutputID))

	chainOutputs := isc.NewChainOutputs(
		anchorOutput,
		anchorOutputID,
		nil,
		iotago.OutputID{},
	)

	return block, chainOutputs, blockIssuerAccountID, chainID, nil
}
