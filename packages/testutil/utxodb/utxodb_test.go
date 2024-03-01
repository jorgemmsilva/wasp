package utxodb_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/builder"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/testutil/utxodb"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/util"
)

func TestRequestFunds(t *testing.T) {
	u := utxodb.New(testutil.L1API)
	wallet, block, err := u.NewWalletWithFundsFromFaucet()
	require.NoError(t, err)
	require.EqualValues(t, u.Supply()-utxodb.FundsFromFaucetAmount, u.GetAddressBalanceBaseTokens(u.GenesisAddress()))
	require.EqualValues(t, utxodb.FundsFromFaucetAmount, u.GetAddressBalanceBaseTokens(wallet.Address()))
	tx := util.TxFromBlock(block)

	txID, err := tx.Transaction.ID()
	require.NoError(t, err)
	require.Same(t, tx, u.MustGetTransaction(txID))
}

func TestAddTransactionFail(t *testing.T) {
	u := utxodb.New(testutil.L1API)

	_, block, err := u.NewWalletWithFundsFromFaucet()
	require.NoError(t, err)

	err = u.AddToLedger(block)
	require.Error(t, err)
}

func TestDoubleSpend(t *testing.T) {
	u := utxodb.New(testutil.L1API)

	wallet1, block1, err := u.NewWalletWithFundsFromFaucet()

	require.NoError(t, err)
	tx1 := util.TxFromBlock(block1)
	tx1ID, err := tx1.Transaction.ID()
	require.NoError(t, err)

	input := tx1.Transaction.Outputs[0]
	inputID := iotago.OutputIDFromTransactionIDAndIndex(tx1ID, 0)
	require.NoError(t, err)

	// faucet gives an account output
	accountOutput := input.Clone().(*iotago.AccountOutput)
	blockIssuerID := accountOutput.AccountID
	accountOutput.Mana = 0
	accountOutput.AccountID = util.AccountIDFromOutputAndID(accountOutput, inputID)

	// transition the accountOutput
	txb2 := u.TxBuilder(wallet1).
		AddInput(&builder.TxInput{
			UnlockTarget: wallet1.Address(),
			Input:        input,
			InputID:      inputID,
		}).
		AddOutput(accountOutput)

	blockSpend2, err := transaction.FinalizeTxAndBuildBlock(
		testutil.L1API,
		txb2,
		u.BlockIssuance(),
		0,
		blockIssuerID,
		wallet1,
	)
	require.NoError(t, err)

	err = u.AddToLedger(blockSpend2)
	require.NoError(t, err)

	// try to double spend the received account output
	txb3 := u.TxBuilder(wallet1).
		AddInput(&builder.TxInput{
			UnlockTarget: wallet1.Address(),
			Input:        input,
			InputID:      inputID,
		}).
		AddOutput(accountOutput)

	blockDoubleSpend, err := transaction.FinalizeTxAndBuildBlock(
		testutil.L1API,
		txb3,
		u.BlockIssuance(),
		0,
		blockIssuerID,
		wallet1,
	)
	require.NoError(t, err)

	err = u.AddToLedger(blockDoubleSpend)
	require.Error(t, err)
}

func TestGetOutput(t *testing.T) {
	u := utxodb.New(testutil.L1API)
	_, block, err := u.NewWalletWithFundsFromFaucet()
	require.NoError(t, err)
	tx := util.TxFromBlock(block)

	txID, err := tx.Transaction.ID()
	require.NoError(t, err)

	outid0 := iotago.OutputIDFromTransactionIDAndIndex(txID, 0)
	out0 := u.GetOutput(outid0)
	require.EqualValues(t, utxodb.FundsFromFaucetAmount, out0.BaseTokenAmount())

	outidFail := iotago.OutputIDFromTransactionIDAndIndex(txID, 1)
	require.Nil(t, u.GetOutput(outidFail))
}
