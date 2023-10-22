package origin

import (
	"encoding/json"
	"fmt"
	"time"

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
// originDeposit must exclude the minSD for the AccountOutput
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

func InitChainByAccountOutput(chainStore state.Store, accountOutput *isc.AccountOutputWithID) (state.Block, error) {
	var initParams dict.Dict
	if originMetadata := accountOutput.GetAccountOutput().FeatureSet().Metadata(); originMetadata != nil {
		var err error
		initParams, err = dict.FromBytes(originMetadata.Data)
		if err != nil {
			return nil, fmt.Errorf("invalid parameters on origin AO, %w", err)
		}
	}
	aoMinSD, err := parameters.RentStructure().MinDeposit(accountOutput.GetAccountOutput())
	if err != nil {
		return nil, err
	}
	commonAccountAmount := accountOutput.GetAccountOutput().Amount - aoMinSD
	originBlock := InitChain(chainStore, initParams, commonAccountAmount)

	originAOStateMetadata, err := transaction.StateMetadataFromBytes(accountOutput.GetStateMetadata())
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
			"l1Commitment mismatch between originAO / originBlock: %s / %s, AOminSD: %d, L1params: %s",
			originAOStateMetadata.L1Commitment,
			originBlock.L1Commitment(),
			aoMinSD,
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
) (*iotago.SignedTransaction, *iotago.AccountOutput, isc.ChainID, error) {
	walletAddr := keyPair.GetPublicKey().AsEd25519Address()

	if initParams == nil {
		initParams = dict.New()
	}
	if initParams.Get(ParamChainOwner) == nil {
		// default chain owner to the gov address
		initParams.Set(ParamChainOwner, isc.NewAgentID(governanceControllerAddress).Bytes())
	}

	accountOutput := &iotago.AccountOutput{
		Amount:        deposit,
		StateMetadata: calcStateMetadata(initParams, deposit, schemaVersion), // NOTE: Updated below.
		Conditions: iotago.AccountOutputUnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{Address: stateControllerAddress},
			&iotago.GovernorAddressUnlockCondition{Address: governanceControllerAddress},
		},
		Features: iotago.AccountOutputFeatures{
			&iotago.MetadataFeature{Data: initParams.Bytes()},
		},
		Mana: depositMana,
	}

	minSD, err := parameters.RentStructure().MinDeposit(accountOutput)
	if err != nil {
		return nil, accountOutput, isc.ChainID{}, err
	}
	minAmount := minSD + governance.DefaultMinBaseTokensOnCommonAccount
	if accountOutput.Amount < minAmount {
		accountOutput.Amount = minAmount
	}
	// update the L1 commitment to not include the minimumSD
	accountOutput.StateMetadata = calcStateMetadata(initParams, accountOutput.Amount-minSD, schemaVersion)

	txInputs, remainder, err := transaction.ComputeInputsAndRemainder(
		walletAddr,
		unspentOutputs,
		transaction.AssetsAndStoredManaFromOutput(accountOutput),
		creationSlot,
	)
	if err != nil {
		return nil, accountOutput, isc.ChainID{}, err
	}
	outputs := iotago.TxEssenceOutputs{accountOutput}
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
	sigs, err := tx.Sign(
		txInputs.OrderedSet(unspentOutputs).MustCommitment(parameters.L1API()),
		keyPair.GetPrivateKey().AddressKeysForEd25519Address(walletAddr),
	)
	if err != nil {
		return nil, accountOutput, isc.ChainID{}, err
	}

	txid, err := tx.ID()
	if err != nil {
		return nil, accountOutput, isc.ChainID{}, err
	}
	chainID := isc.ChainIDFromAccountID(iotago.AccountIDFromOutputID(iotago.OutputIDFromTransactionIDAndIndex(txid, 0)))

	return &iotago.SignedTransaction{
		API:         parameters.L1API(),
		Transaction: tx,
		Unlocks:     transaction.MakeSignatureAndReferenceUnlocks(len(txInputs), sigs[0]),
	}, accountOutput, chainID, nil
}
