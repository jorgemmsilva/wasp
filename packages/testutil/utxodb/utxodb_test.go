package utxodb

import (
	"testing"

	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/builder"
	"github.com/iotaledger/iota.go/v4/tpkg"
	"github.com/iotaledger/iota.go/v4/vm"
	"github.com/iotaledger/wasp/packages/parameters"
)

func TestRequestFunds(t *testing.T) {
	u := New(parameters.L1API())
	addr := tpkg.RandEd25519Address()
	tx, err := u.GetFundsFromFaucet(addr)
	require.NoError(t, err)
	require.EqualValues(t, u.Supply()-FundsFromFaucetAmount, u.GetAddressBalanceBaseTokens(u.GenesisAddress()))
	require.EqualValues(t, FundsFromFaucetAmount, u.GetAddressBalanceBaseTokens(addr))

	txID, err := tx.Transaction.ID()
	require.NoError(t, err)
	require.Same(t, tx, u.MustGetTransaction(txID))
}

func TestAddTransactionFail(t *testing.T) {
	u := New(parameters.L1API())

	addr := tpkg.RandEd25519Address()
	tx, err := u.GetFundsFromFaucet(addr)
	require.NoError(t, err)

	err = u.AddToLedger(tx)
	require.Error(t, err)
}

func TestDoubleSpend(t *testing.T) {
	_, addr1, addrKeys := tpkg.RandEd25519Identity()
	key1Signer := iotago.NewInMemoryAddressSigner(addrKeys)

	addr2 := tpkg.RandEd25519Address()
	addr3 := tpkg.RandEd25519Address()

	u := New(parameters.L1API())

	tx1, err := u.GetFundsFromFaucet(addr1)
	require.NoError(t, err)
	tx1ID, err := tx1.Transaction.ID()
	require.NoError(t, err)

	input := tx1.Transaction.Outputs[0]
	inputID := iotago.OutputIDFromTransactionIDAndIndex(tx1ID, 0)
	mana, err := vm.TotalManaIn(
		u.api.ManaDecayProvider(),
		u.api.StorageScoreStructure(),
		u.SlotIndex(),
		vm.InputSet{inputID: input},
		vm.RewardsInputSet{},
	)

	spend2, err := u.TxBuilder().
		AddInput(&builder.TxInput{
			UnlockTarget: addr1,
			Input:        input,
			InputID:      inputID,
		}).
		AddOutput(&iotago.BasicOutput{
			Amount: FundsFromFaucetAmount,
			UnlockConditions: iotago.BasicOutputUnlockConditions{
				&iotago.AddressUnlockCondition{Address: addr2},
			},
			Mana: mana,
		}).
		Build(key1Signer)
	require.NoError(t, err)
	err = u.AddToLedger(spend2)
	require.NoError(t, err)

	spend3, err := u.TxBuilder().
		AddInput(&builder.TxInput{
			UnlockTarget: addr1,
			Input:        input,
			InputID:      inputID,
		}).
		AddOutput(&iotago.BasicOutput{
			Amount: FundsFromFaucetAmount,
			UnlockConditions: iotago.BasicOutputUnlockConditions{
				&iotago.AddressUnlockCondition{Address: addr3},
			},
			Mana: mana,
		}).
		Build(key1Signer)
	require.NoError(t, err)
	err = u.AddToLedger(spend3)
	require.Error(t, err)
}

func TestGetOutput(t *testing.T) {
	u := New(parameters.L1API())
	addr := tpkg.RandEd25519Address()
	tx, err := u.GetFundsFromFaucet(addr)
	require.NoError(t, err)

	txID, err := tx.Transaction.ID()
	require.NoError(t, err)

	outid0 := iotago.OutputIDFromTransactionIDAndIndex(txID, 0)
	out0 := u.GetOutput(outid0)
	require.EqualValues(t, FundsFromFaucetAmount, out0.BaseTokenAmount())

	outid1 := iotago.OutputIDFromTransactionIDAndIndex(txID, 1)
	out1 := u.GetOutput(outid1)
	require.EqualValues(t, u.Supply()-FundsFromFaucetAmount, out1.BaseTokenAmount())

	outidFail := iotago.OutputIDFromTransactionIDAndIndex(txID, 5)
	require.Nil(t, u.GetOutput(outidFail))
}
