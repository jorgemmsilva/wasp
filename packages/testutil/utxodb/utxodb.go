package utxodb

import (
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sync"
	"time"

	"github.com/samber/lo"

	hiveEd25519 "github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/iota.go/v4/builder"
	"github.com/iotaledger/iota.go/v4/tpkg"
	"github.com/iotaledger/iota.go/v4/vm"
	"github.com/iotaledger/iota.go/v4/vm/nova"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/util"
)

const (
	// FundsFromFaucetAmount is how many base tokens are returned from the faucet.
	FundsFromFaucetAmount = iotago.BaseToken(1_000_000_000)
	ManaFromFaucetAmount  = iotago.Mana(1_000_000_000)
)

var (
	genesisPrivKey = ed25519.NewKeyFromSeed([]byte("3.141592653589793238462643383279"))
	genesisAddress = iotago.Ed25519AddressFromPubKey(genesisPrivKey.Public().(ed25519.PublicKey))
	genesisSigner  = cryptolib.KeyPairFromPrivateKey(lo.Must(cryptolib.PrivateKeyFromBytes(genesisPrivKey[:])))
)

// UtxoDB mocks the Tangle ledger by implementing a fully synchronous in-memory database
// of transactions. It ensures the consistency of the ledger and all added transactions
// by checking inputs, outputs and signatures.
type UtxoDB struct {
	mutex              sync.RWMutex
	transactions       map[iotago.TransactionID]*iotago.SignedTransaction
	utxo               map[iotago.OutputID]struct{}
	blockIssuer        map[iotago.AccountID]struct{}
	timestamp          time.Time
	timestep           time.Duration
	api                iotago.API
	genesisBlockIssuer iotago.AccountID
}

// New creates a new UtxoDB instance
func New(api iotago.API) *UtxoDB {
	u := &UtxoDB{
		transactions: make(map[iotago.TransactionID]*iotago.SignedTransaction),
		utxo:         make(map[iotago.OutputID]struct{}),
		blockIssuer:  make(map[iotago.AccountID]struct{}),
		timestamp:    time.Unix(1, 0),
		timestep:     1 * time.Millisecond,
		api:          api,
	}
	u.genesisInit()
	return u
}

func (u *UtxoDB) TxBuilder(signer iotago.AddressSigner) *builder.TransactionBuilder {
	return builder.NewTransactionBuilder(u.api, signer).SetCreationSlot(u.SlotIndex())
}

func (u *UtxoDB) genesisInit() {
	inputID := u.dummyOutputID()
	blockIssuerAccountID := iotago.AccountIDFromOutputID(inputID)

	hivepubKey, _, err := hiveEd25519.PublicKeyFromBytes(genesisPrivKey.Public().(ed25519.PublicKey)[:])
	if err != nil {
		panic(err)
	}

	// create and account output for the faucet to be able to issue blocks
	accountIssuerOutput := &iotago.AccountOutput{
		Amount:         0,
		Mana:           0,
		AccountID:      blockIssuerAccountID,
		FoundryCounter: 0,
		UnlockConditions: []iotago.AccountOutputUnlockCondition{
			&iotago.AddressUnlockCondition{
				Address: genesisAddress,
			},
		},
		Features: []iotago.AccountOutputFeature{
			&iotago.BlockIssuerFeature{
				ExpirySlot: math.MaxUint32,
				BlockIssuerKeys: []iotago.BlockIssuerKey{
					iotago.Ed25519PublicKeyHashBlockIssuerKeyFromPublicKey(hivepubKey),
				},
			},
		},
	}
	accountIssuerMinSD := lo.Must(testutil.L1API.StorageScoreStructure().MinDeposit(accountIssuerOutput))
	accountIssuerOutput.Amount = accountIssuerMinSD

	// keep all the iotas and mana in a basic output (so it can be freely sent to someone else)
	fundsOutput := &iotago.BasicOutput{
		Amount: u.Supply() - accountIssuerMinSD,
		Mana:   iotago.MaxMana / 2, // if you use max mana here, it will blow up when trying to build a block later
		UnlockConditions: iotago.BasicOutputUnlockConditions{
			&iotago.AddressUnlockCondition{Address: genesisAddress},
		},
	}

	genesisTx, err := u.TxBuilder(genesisSigner).
		AddInput(&builder.TxInput{
			UnlockTarget: genesisAddress,
			InputID:      inputID,
			Input: &iotago.BasicOutput{
				Amount: u.Supply(),
				Mana:   iotago.MaxMana / 2, // if you use max mana here, it will blow up when trying to build a block later
				UnlockConditions: iotago.BasicOutputUnlockConditions{
					&iotago.AddressUnlockCondition{Address: genesisAddress},
				},
			},
		}).
		AddOutput(accountIssuerOutput).
		AddOutput(fundsOutput).
		Build()
	if err != nil {
		panic(err)
	}
	u.addTransaction(genesisTx, true)
	u.genesisBlockIssuer = blockIssuerAccountID
}

func (u *UtxoDB) dummyOutputID() iotago.OutputID {
	var txID iotago.TransactionID
	binary.LittleEndian.PutUint32(txID[iotago.IdentifierLength:iotago.TransactionIDLength], uint32(u.SlotIndex()))
	var outputID iotago.OutputID
	copy(outputID[:], txID[:])
	binary.LittleEndian.PutUint16(outputID[iotago.TransactionIDLength:], 0) // tx index 0
	return outputID
}

func hasImplicitAccount(o *iotago.BasicOutput) bool {
	if addressUnlock := o.UnlockConditionSet().Address(); addressUnlock != nil {
		return addressUnlock.Address.Type() == iotago.AddressImplicitAccountCreation
	}
	return false
}

func (u *UtxoDB) addTransaction(tx *iotago.SignedTransaction, isGenesis bool) {
	txid, err := tx.Transaction.ID()
	if err != nil {
		panic(err)
	}
	// delete consumed account outputs from the ledger (they will be added back if still on the output side)
	inputs, err := u.getTransactionInputs(tx)
	if !isGenesis && err != nil {
		panic(err)
	}
	for outID, out := range inputs {
		delete(u.utxo, outID)
		switch o := out.(type) {
		case *iotago.AccountOutput:
			delete(u.blockIssuer, util.AccountIDFromOutputAndID(o, outID))
		case *iotago.BasicOutput: // check for implicit accounts
			if hasImplicitAccount(o) {
				delete(u.blockIssuer, iotago.AccountIDFromOutputID(outID))
			}
		}
	}
	// store transaction
	u.transactions[txid] = tx

	// add unspent outputs to the ledger
	for i, out := range tx.Transaction.Outputs {
		outputID := iotago.OutputIDFromTransactionIDAndIndex(txid, uint16(i))
		u.utxo[outputID] = struct{}{}

		// keep track of issuer accounts
		switch o := out.(type) {
		case *iotago.AccountOutput:
			u.blockIssuer[util.AccountIDFromOutputAndID(o, outputID)] = struct{}{}
		case *iotago.BasicOutput: // check for implicit accounts
			if hasImplicitAccount(o) {
				u.blockIssuer[iotago.AccountIDFromOutputID(outputID)] = struct{}{}
			}
		}
	}
	u.advanceTime(u.timestep)
	u.checkLedgerBalance()
}

func (u *UtxoDB) advanceTime(timeStep time.Duration) {
	u.timestamp = u.timestamp.Add(timeStep)
}

func (u *UtxoDB) AdvanceTime(timeStep time.Duration) {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	u.advanceTime(timeStep)
}

func (u *UtxoDB) Timestamp() time.Time {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	return u.timestamp
}

func (u *UtxoDB) SetTimestep(ts time.Duration) {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	u.timestep = ts
}

func (u *UtxoDB) SlotIndex() iotago.SlotIndex {
	return u.api.TimeProvider().SlotFromTime(u.Timestamp())
}

func (u *UtxoDB) slotIndex() iotago.SlotIndex {
	return u.api.TimeProvider().SlotFromTime(u.timestamp)
}

// GenesisAddress returns the genesis address.
func (u *UtxoDB) GenesisAddress() iotago.Address {
	return genesisAddress
}

// NewWalletWithFundsFromFaucet sends FundsFromFaucetAmount base tokens from the genesis address to the given address.
//
//nolint:funlen
func (u *UtxoDB) NewWalletWithFundsFromFaucet(keyPair ...*cryptolib.KeyPair) (*cryptolib.KeyPair, *iotago.Block, error) {
	var wallet *cryptolib.KeyPair
	if len(keyPair) > 0 {
		wallet = keyPair[0]
	} else {
		wallet = cryptolib.NewKeyPair()
	}
	walletPubkey := wallet.GetPublicKey()
	if len(u.getUnspentOutputs(wallet.Address())) > 0 {
		return nil, nil, fmt.Errorf("this account already has funds on L1")
	}

	implicitAccoutAddr := iotago.ImplicitAccountCreationAddressFromPubKey(walletPubkey.AsEd25519PubKey())

	hivePubKey, _, err := hiveEd25519.PublicKeyFromBytes((*walletPubkey)[:])
	if err != nil {
		return nil, nil, err
	}

	unspentOutputs := u.getUnspentOutputs(genesisAddress)
	if len(unspentOutputs) != 2 {
		panic("number of genesis outputs must be 2")
	}
	var input *iotago.BasicOutput
	var inputID iotago.OutputID
	for oid, out := range unspentOutputs {
		if out.Type() == iotago.OutputAccount {
			continue
		}
		input = out.(*iotago.BasicOutput)
		inputID = oid
	}

	txBuilder := u.TxBuilder(genesisSigner).
		AddInput(&builder.TxInput{
			UnlockTarget: genesisAddress,
			InputID:      inputID,
			Input:        input,
		}).
		AddOutput(&iotago.BasicOutput{
			Amount: FundsFromFaucetAmount,
			Mana:   ManaFromFaucetAmount,
			UnlockConditions: []iotago.BasicOutputUnlockCondition{
				&iotago.AddressUnlockCondition{Address: implicitAccoutAddr}, // send to the implicit acount
			},
			Features: []iotago.BasicOutputFeature{},
		})

	nextFaucetOutput := input.Clone().(*iotago.BasicOutput)
	nextFaucetOutput.Amount -= FundsFromFaucetAmount
	nextFaucetOutput.Mana = 0 // set to 0, because it will be filled by `FinalizeAndSignTx`
	txBuilder.AddOutput(nextFaucetOutput)

	blockIssuance := u.BlockIssuance()

	block, err := transaction.FinalizeTxAndBuildBlock(
		testutil.L1API,
		txBuilder,
		blockIssuance,
		1,
		u.genesisBlockIssuer,
		genesisSigner,
	)
	if err != nil {
		return nil, nil, err
	}

	err = u.AddToLedger(block)
	if err != nil {
		return nil, nil, err
	}

	// now take the basic output owned by the implicit acount and convert it to an AccountOutput
	tx := util.TxFromBlock(block)
	outputToConvert := tx.Transaction.Outputs[0].Clone()
	outputToConvertID := iotago.OutputIDFromTransactionIDAndIndex(lo.Must(tx.Transaction.ID()), 0)

	txBuilderTarget := u.TxBuilder(wallet).
		AddInput(&builder.TxInput{
			UnlockTarget: wallet.Address(),
			InputID:      outputToConvertID,
			Input:        outputToConvert,
		})

	blockIssuerAccountID := iotago.AccountIDFromOutputID(outputToConvertID)

	txBuilderTarget.
		AddOutput(&iotago.AccountOutput{
			Amount:    FundsFromFaucetAmount,
			Mana:      0,
			AccountID: blockIssuerAccountID,
			UnlockConditions: []iotago.AccountOutputUnlockCondition{
				&iotago.AddressUnlockCondition{
					Address: wallet.Address(),
				},
			},
			Features: []iotago.AccountOutputFeature{
				&iotago.BlockIssuerFeature{
					ExpirySlot: math.MaxUint32,
					BlockIssuerKeys: []iotago.BlockIssuerKey{
						iotago.Ed25519PublicKeyHashBlockIssuerKeyFromPublicKey(hivePubKey),
					},
				},
			},
		})

	convertBlock, err := transaction.FinalizeTxAndBuildBlock(
		testutil.L1API,
		txBuilderTarget,
		u.BlockIssuance(),
		0,
		blockIssuerAccountID,
		wallet,
	)
	if err != nil {
		return nil, nil, err
	}

	err = u.AddToLedger(convertBlock)
	if err != nil {
		return nil, nil, err
	}

	return wallet, convertBlock, nil
}

// Supply returns supply of the instance.
func (u *UtxoDB) Supply() iotago.BaseToken {
	return u.api.ProtocolParameters().TokenSupply()
}

// GetOutput finds an output by ID (either spent or unspent).
func (u *UtxoDB) GetOutput(outID iotago.OutputID) iotago.Output {
	u.mutex.RLock()
	defer u.mutex.RUnlock()
	return u.getOutput(outID)
}

func (u *UtxoDB) getOutput(outputID iotago.OutputID) iotago.Output {
	tx, ok := u.getTransaction(outputID.TransactionID())
	if !ok {
		return nil
	}
	if int(outputID.Index()) >= len(tx.Transaction.Outputs) {
		return nil
	}
	return tx.Transaction.Outputs[outputID.Index()]
}

func (u *UtxoDB) getTransactionInputs(tx *iotago.SignedTransaction) (iotago.OutputSet, error) {
	inputs := iotago.OutputSet{}
	utxoInputs, err := tx.Transaction.Inputs()
	if err != nil {
		panic(err)
	}
	for _, input := range utxoInputs {
		outputID := input.OutputID()
		output := u.getOutput(outputID)
		if output == nil {
			return nil, errors.New("output not found")
		}
		inputs[outputID] = output
	}
	return inputs, nil
}

var novaVM = nova.NewVirtualMachine()

func (u *UtxoDB) validateTransaction(tx *iotago.SignedTransaction) error {
	// serialize for syntactic check
	if _, err := u.api.Encode(tx, serix.WithValidation()); err != nil {
		return fmt.Errorf("validateTransaction: encode: %w", err)
	}

	inputs, err := u.getTransactionInputs(tx)
	if err != nil {
		return err
	}
	for outID := range inputs {
		if _, ok := u.utxo[outID]; !ok {
			return fmt.Errorf("referenced output is not unspent: %s", outID.ToHex())
		}
	}

	slot := u.slotIndex()
	resolvedInputs := vm.ResolvedInputs{
		InputSet: vm.InputSet(inputs),
		CommitmentInput: iotago.NewCommitment(
			u.api.Version(),
			slot,
			iotago.NewCommitmentID(slot-1, tpkg.Rand32ByteArray()),
			tpkg.Rand32ByteArray(),
			tpkg.RandUint64(math.MaxUint64),
			tpkg.RandMana(iotago.MaxMana),
		),
		BlockIssuanceCreditInputSet: vm.BlockIssuanceCreditInputSet{},
	}
	for _, bic := range lo.Must(tx.Transaction.BICInputs()) {
		resolvedInputs.BlockIssuanceCreditInputSet[bic.AccountID] = 0
	}

	unlockedIdentities, err := novaVM.ValidateUnlocks(tx, resolvedInputs)
	if err != nil {
		return fmt.Errorf("validateTransaction: ValidateUnlocks: %w", err)
	}

	_, err = novaVM.Execute(tx.Transaction, resolvedInputs, unlockedIdentities)
	if err != nil {
		return fmt.Errorf("validateTransaction: Execute: %w", err)
	}

	return nil
}

// AddToLedger adds a transaction to UtxoDB, ensuring consistency of the UtxoDB ledger.
func (u *UtxoDB) AddToLedger(block *iotago.Block) error {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	// check block signature
	valid, err := block.VerifySignature()
	if err != nil || !valid {
		return fmt.Errorf("invalid block signature")
	}

	// verify that there is an account issuer for the block
	if _, ok := u.blockIssuer[block.Header.IssuerID]; !ok {
		return fmt.Errorf("block issuer not found")
	}

	// check block mana
	b, ok := block.Body.(*iotago.BasicBlockBody)
	if !ok {
		return fmt.Errorf("unexpected block type, %v", block.Body.Type())
	}
	manaCost, err := block.ManaCost(u.blockIssuanceInfoFromSlot(block.Header.SlotCommitmentID.Index()).LatestCommitment.ReferenceManaCost)
	if err != nil {
		return err
	}
	if b.MaxBurnedMana < manaCost {
		return fmt.Errorf("not enough mana burned: expected: %v, burned %v", manaCost, b.MaxBurnedMana)
	}

	// check block tx
	if b.Payload.PayloadType() != iotago.PayloadSignedTransaction {
		return fmt.Errorf("unexpected payload type, %v", b.Payload.PayloadType())
	}

	tx, _ := b.Payload.(*iotago.SignedTransaction)
	if err := u.validateTransaction(tx); err != nil {
		return err
	}
	u.addTransaction(tx, false)

	return nil
}

// GetTransaction retrieves value transaction by its hash (ID).
func (u *UtxoDB) GetTransaction(txID iotago.TransactionID) (*iotago.SignedTransaction, bool) {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	return u.getTransaction(txID)
}

// MustGetTransaction same as GetTransaction only panics if transaction is not in UtxoDB.
func (u *UtxoDB) MustGetTransaction(txID iotago.TransactionID) *iotago.SignedTransaction {
	u.mutex.RLock()
	defer u.mutex.RUnlock()
	return u.mustGetTransaction(txID)
}

// GetUnspentOutputs returns all unspent outputs locked by the address with its ids
func (u *UtxoDB) GetUnspentOutputs(addr iotago.Address) iotago.OutputSet {
	u.mutex.RLock()
	defer u.mutex.RUnlock()
	return u.getUnspentOutputs(addr)
}

// GetAddressBalanceBaseTokens returns the total amount of base tokens owned by the address
func (u *UtxoDB) GetAddressBalanceBaseTokens(addr iotago.Address) iotago.BaseToken {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	ret := iotago.BaseToken(0)
	for _, out := range u.getUnspentOutputs(addr) {
		ret += out.BaseTokenAmount()
	}
	return ret
}

// GetAddressBalanceNativeTokens returns the total amount of native tokens owned by the address
func (u *UtxoDB) GetAddressBalanceNativeTokens(addr iotago.Address) iotago.NativeTokenSum {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	tokens := iotago.NativeTokenSum{}
	for _, out := range u.getUnspentOutputs(addr) {
		if out.FeatureSet().HasNativeTokenFeature() {
			token := out.FeatureSet().NativeToken()
			val := tokens[token.ID]
			if val == nil {
				val = new(big.Int)
			}
			tokens[token.ID] = new(big.Int).Add(val, token.Amount)
		}
	}
	return tokens
}

func (u *UtxoDB) GetAddressBalanceNFTs(addr iotago.Address) iotago.NFTIDs {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	var nfts iotago.NFTIDs
	for _, out := range u.getUnspentOutputs(addr) {
		if out.Type() == iotago.OutputNFT {
			nfts = append(nfts, out.(*iotago.NFTOutput).NFTID)
		}
	}
	return nfts
}

// GetAnchorOutputs collects all outputs of type AnchorOutput for the address
func (u *UtxoDB) GetAnchorOutputs(addr iotago.Address) map[iotago.OutputID]*iotago.AnchorOutput {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	return filterOutputsOfType[*iotago.AnchorOutput](u.getUnspentOutputs(addr))
}

// GetAccountOutputs collects all outputs of type AccountOutput for the address
func (u *UtxoDB) GetAccountOutputs(addr iotago.Address) map[iotago.OutputID]*iotago.AccountOutput {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	return filterOutputsOfType[*iotago.AccountOutput](u.getUnspentOutputs(addr))
}

var dummyblockIDBytes = [iotago.BlockHeaderLength + iotago.Ed25519SignatureSerializedBytesSize]byte{0x1}

func (u *UtxoDB) blockIssuanceInfoFromSlot(si iotago.SlotIndex) *api.IssuanceBlockHeaderResponse {
	minRefManaCost := testutil.L1API.ProtocolParameters().CongestionControlParameters().MinReferenceManaCost

	dummyBlockID := iotago.NewBlockID(si, lo.Must(iotago.BlockIdentifierFromBlockBytes(dummyblockIDBytes[:])))

	return &api.IssuanceBlockHeaderResponse{
		StrongParents:                []iotago.BlockID{dummyBlockID},
		WeakParents:                  []iotago.BlockID{dummyBlockID},
		ShallowLikeParents:           []iotago.BlockID{dummyBlockID},
		LatestParentBlockIssuingTime: u.timestamp,
		LatestFinalizedSlot:          si,
		LatestCommitment: &iotago.Commitment{
			ProtocolVersion:      3,
			Slot:                 si,
			PreviousCommitmentID: [36]byte{},
			RootsID:              [32]byte{},
			CumulativeWeight:     0,
			ReferenceManaCost:    minRefManaCost + iotago.Mana(si), // make mana cost increase every block
		},
	}
}

func (u *UtxoDB) BlockIssuance() *api.IssuanceBlockHeaderResponse {
	return u.blockIssuanceInfoFromSlot(u.slotIndex())
}

func filterOutputsOfType[T iotago.Output](outs iotago.OutputSet) map[iotago.OutputID]T {
	ret := make(map[iotago.OutputID]T)
	for oid, out := range outs {
		if o, ok := out.(T); ok {
			ret[oid] = o
		}
	}
	return ret
}

func (u *UtxoDB) GetAddressNFTs(addr iotago.Address) map[iotago.OutputID]*iotago.NFTOutput {
	outs := u.getUnspentOutputs(addr)
	ret := make(map[iotago.OutputID]*iotago.NFTOutput)
	for oid, out := range outs {
		if o, ok := out.(*iotago.NFTOutput); ok {
			ret[oid] = o
		}
	}
	return ret
}

func (u *UtxoDB) getTransaction(txID iotago.TransactionID) (*iotago.SignedTransaction, bool) {
	tx, ok := u.transactions[txID]
	if !ok {
		return nil, false
	}
	return tx, true
}

func (u *UtxoDB) mustGetTransaction(txID iotago.TransactionID) *iotago.SignedTransaction {
	tx, ok := u.getTransaction(txID)
	if !ok {
		panic(fmt.Errorf("utxodb.mustGetTransaction: tx id doesn't exist: %s", txID))
	}
	return tx
}

func getOutputAddress(out iotago.Output, outputID iotago.OutputID) iotago.Address {
	switch output := out.(type) {
	case iotago.OwnerTransitionIndependentOutput:
		return output.Owner()
	case iotago.OwnerTransitionDependentOutput:
		chainID := output.ChainID()
		if chainID.Empty() {
			utxoChainID, is := chainID.(iotago.UTXOIDChainID)
			if !is {
				panic("unknown ChainID type")
			}
			//nolint:forcetypeassert // we can safely assume that this is an UTXOInput
			chainID = utxoChainID.FromOutputID(outputID)
		}
		return chainID.ToAddress()
	default:
		panic("unknown ident output type")
	}
}

func (u *UtxoDB) getUnspentOutputs(addr iotago.Address) iotago.OutputSet {
	ret := make(iotago.OutputSet)
	for outputID := range u.utxo {
		output := u.getOutput(outputID)
		if getOutputAddress(output, outputID).Equal(addr) {
			ret[outputID] = output
		}
	}
	return ret
}

func (u *UtxoDB) checkLedgerBalance() {
	total := iotago.BaseToken(0)
	for outID := range u.utxo {
		out := u.getOutput(outID)
		total += out.BaseTokenAmount()
	}
	if total != u.Supply() {
		panic("utxodb: wrong ledger balance")
	}
}

type UtxoDBState struct {
	Transactions map[string]*iotago.SignedTransaction
	UTXO         []string
	Timestamp    time.Time
	Timestep     time.Duration
}

func (u *UtxoDB) State() *UtxoDBState {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	txs := make(map[string]*iotago.SignedTransaction)
	for txid, tx := range u.transactions {
		txs[hex.EncodeToString(txid[:])] = tx
	}

	utxo := make([]string, 0, len(u.utxo))
	for oid := range u.utxo {
		utxo = append(utxo, hex.EncodeToString(oid[:]))
	}

	return &UtxoDBState{
		Transactions: txs,
		UTXO:         utxo,
		Timestamp:    u.timestamp,
		Timestep:     u.timestep,
	}
}

func (u *UtxoDB) SetState(state *UtxoDBState) {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	u.transactions = make(map[iotago.TransactionID]*iotago.SignedTransaction)
	u.utxo = make(map[iotago.OutputID]struct{})
	u.timestamp = state.Timestamp
	u.timestep = state.Timestep

	for s, tx := range state.Transactions {
		b, err := hex.DecodeString(s)
		if err != nil {
			panic(err)
		}
		var txid iotago.TransactionID
		copy(txid[:], b)
		u.transactions[txid] = tx
	}
	for _, s := range state.UTXO {
		b, err := hex.DecodeString(s)
		if err != nil {
			panic(err)
		}
		var oid iotago.OutputID
		copy(oid[:], b)
		u.utxo[oid] = struct{}{}
	}
}
