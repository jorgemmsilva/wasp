package utxodb

import (
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/iotaledger/hive.go/serializer/v2/serix"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/builder"
	"github.com/iotaledger/iota.go/v4/tpkg"
	"github.com/iotaledger/iota.go/v4/vm"
	"github.com/iotaledger/iota.go/v4/vm/nova"
)

const (
	// FundsFromFaucetAmount is how many base tokens are returned from the faucet.
	FundsFromFaucetAmount = iotago.BaseToken(1_000_000_000)
	ManaFromFaucetAmount  = iotago.Mana(1_000_000_000)
)

var (
	genesisPrivKey = ed25519.NewKeyFromSeed([]byte("3.141592653589793238462643383279"))
	genesisAddress = iotago.Ed25519AddressFromPubKey(genesisPrivKey.Public().(ed25519.PublicKey))
	genesisSigner  = iotago.NewInMemoryAddressSigner(iotago.NewAddressKeysForEd25519Address(genesisAddress, genesisPrivKey))
)

// UtxoDB mocks the Tangle ledger by implementing a fully synchronous in-memory database
// of transactions. It ensures the consistency of the ledger and all added transactions
// by checking inputs, outputs and signatures.
type UtxoDB struct {
	mutex        sync.RWMutex
	transactions map[iotago.TransactionID]*iotago.SignedTransaction
	utxo         map[iotago.OutputID]struct{}
	slotIndex    iotago.SlotIndex
	api          iotago.API
}

// New creates a new UtxoDB instance
func New(l1api ...iotago.API) *UtxoDB {
	u := &UtxoDB{
		transactions: make(map[iotago.TransactionID]*iotago.SignedTransaction),
		utxo:         make(map[iotago.OutputID]struct{}),
		api: func() iotago.API {
			if len(l1api) > 0 {
				return l1api[0]
			}
			return tpkg.TestAPI
		}(),
	}
	u.genesisInit()
	return u
}

func (u *UtxoDB) TxBuilder() *builder.TransactionBuilder {
	return builder.NewTransactionBuilder(u.api).SetCreationSlot(u.slotIndex)
}

func (u *UtxoDB) genesisInit() {
	genesisTx, err := u.TxBuilder().
		AddInput(&builder.TxInput{
			UnlockTarget: genesisAddress,
			InputID:      u.dummyOutputID(),
			Input: &iotago.BasicOutput{
				Amount: u.Supply(),
				Mana:   iotago.MaxMana / 2,
				Conditions: iotago.BasicOutputUnlockConditions{
					&iotago.AddressUnlockCondition{Address: genesisAddress},
				},
			},
		}).
		AddOutput(&iotago.BasicOutput{
			Amount: u.Supply(),
			Mana:   iotago.MaxMana / 2,
			Conditions: iotago.BasicOutputUnlockConditions{
				&iotago.AddressUnlockCondition{Address: genesisAddress},
			},
		}).
		Build(genesisSigner)
	if err != nil {
		panic(err)
	}
	u.addTransaction(genesisTx, true)
}

func (u *UtxoDB) dummyOutputID() iotago.OutputID {
	var txID iotago.TransactionID
	binary.LittleEndian.PutUint32(txID[iotago.IdentifierLength:iotago.TransactionIDLength], uint32(u.slotIndex))
	var outputID iotago.OutputID
	copy(outputID[:], txID[:])
	binary.LittleEndian.PutUint16(outputID[iotago.TransactionIDLength:], 0) // tx index 0
	return outputID
}

func (u *UtxoDB) addTransaction(tx *iotago.SignedTransaction, isGenesis bool) {
	txid, err := tx.Transaction.ID()
	if err != nil {
		panic(err)
	}
	// delete consumed outputs from the ledger
	inputs, err := u.getTransactionInputs(tx)
	if !isGenesis && err != nil {
		panic(err)
	}
	for outID := range inputs {
		delete(u.utxo, outID)
	}
	// store transaction
	u.transactions[txid] = tx

	// add unspent outputs to the ledger
	for i := range tx.Transaction.Outputs {
		outputID := iotago.OutputIDFromTransactionIDAndIndex(txid, uint16(i))
		u.utxo[outputID] = struct{}{}
	}
	u.advanceSlotIndex(1)
	u.checkLedgerBalance()
}

func (u *UtxoDB) advanceSlotIndex(step uint) {
	u.slotIndex += iotago.SlotIndex(step)
}

func (u *UtxoDB) AdvanceSlotIndex(step uint) {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	u.advanceSlotIndex(step)
}

func (u *UtxoDB) SlotIndex() iotago.SlotIndex {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	return u.slotIndex
}

// GenesisAddress returns the genesis address.
func (u *UtxoDB) GenesisAddress() iotago.Address {
	return genesisAddress
}

func (u *UtxoDB) mustGetFundsFromFaucetTx(target iotago.Address, amount ...iotago.BaseToken) *iotago.SignedTransaction {
	unspentOutputs := u.getUnspentOutputs(genesisAddress)
	if len(unspentOutputs) != 1 {
		panic("number of genesis outputs must be 1")
	}
	var input *iotago.BasicOutput
	var inputID iotago.OutputID
	for oid, out := range unspentOutputs {
		input = out.(*iotago.BasicOutput)
		inputID = oid
	}

	mana, err := vm.TotalManaIn(
		u.api.ManaDecayProvider(),
		u.api.RentStructure(),
		u.slotIndex,
		vm.InputSet{inputID: input},
	)
	if err != nil {
		panic(err)
	}

	fundsAmount := FundsFromFaucetAmount
	if len(amount) > 0 {
		fundsAmount = amount[0]
	}

	tx, err := u.TxBuilder().
		AddInput(&builder.TxInput{
			UnlockTarget: genesisAddress,
			InputID:      inputID,
			Input:        input,
		}).
		AddOutput(&iotago.BasicOutput{
			Amount: fundsAmount,
			Conditions: iotago.BasicOutputUnlockConditions{
				&iotago.AddressUnlockCondition{Address: target},
			},
			Mana: ManaFromFaucetAmount,
		}).
		AddOutput(&iotago.BasicOutput{
			Amount: input.Amount - fundsAmount,
			Conditions: iotago.BasicOutputUnlockConditions{
				&iotago.AddressUnlockCondition{Address: genesisAddress},
			},
			Mana: mana - ManaFromFaucetAmount,
		}).
		Build(genesisSigner)
	if err != nil {
		panic(err)
	}
	return tx
}

// GetFundsFromFaucet sends FundsFromFaucetAmount base tokens from the genesis address to the given address.
func (u *UtxoDB) GetFundsFromFaucet(target iotago.Address, amount ...iotago.BaseToken) (*iotago.SignedTransaction, error) {
	tx := u.mustGetFundsFromFaucetTx(target, amount...)
	return tx, u.AddToLedger(tx)
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
		return err
	}

	inputs, err := u.getTransactionInputs(tx)
	if err != nil {
		return err
	}
	for outID := range inputs {
		if _, ok := u.utxo[outID]; !ok {
			return errors.New("referenced output is not unspent")
		}
	}

	resolvedInputs := vm.ResolvedInputs{InputSet: vm.InputSet(inputs)}

	unlockedIdentities, err := novaVM.ValidateUnlocks(tx, resolvedInputs)
	if err != nil {
		return err
	}

	_, err = novaVM.Execute(tx.Transaction, resolvedInputs, unlockedIdentities)
	if err != nil {
		return err
	}

	return nil
}

// AddToLedger adds a transaction to UtxoDB, ensuring consistency of the UtxoDB ledger.
func (u *UtxoDB) AddToLedger(tx *iotago.SignedTransaction) error {
	u.mutex.Lock()
	defer u.mutex.Unlock()

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

// GetAccountOutputs collects all outputs of type AccountOutput for the address
func (u *UtxoDB) GetAccountOutputs(addr iotago.Address) map[iotago.OutputID]*iotago.AccountOutput {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	return filterOutputsOfType[*iotago.AccountOutput](u.getUnspentOutputs(addr))
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
	case iotago.TransIndepIdentOutput:
		return output.Ident()
	case iotago.TransDepIdentOutput:
		chainID := output.ChainID()
		if chainID.Empty() {
			chainID = iotago.AccountIDFromOutputID(outputID)
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
	SlotIndex    iotago.SlotIndex
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
		SlotIndex:    u.slotIndex,
	}
}

func (u *UtxoDB) SetState(state *UtxoDBState) {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	u.transactions = make(map[iotago.TransactionID]*iotago.SignedTransaction)
	u.utxo = make(map[iotago.OutputID]struct{})
	u.slotIndex = state.SlotIndex

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
