// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/builder"
	"github.com/iotaledger/iota.go/v4/nodeclient"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/l1connection"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/util"
)

func TestPrivtangleStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping privtangle test in short mode")
	}
	l1.StartPrivtangleIfNecessary(t.Logf)

	// pvt tangle is already stated by the cluster l1_init
	ctx := context.Background()

	//
	// Try call the faucet.
	myKeyPair := cryptolib.NewKeyPair()
	myAddress := myKeyPair.Address()

	nc, err := nodeclient.New(l1.Config.APIAddress)
	require.NoError(t, err)
	_, err = nc.Info(ctx)
	require.NoError(t, err)

	log := testlogger.NewSilentLogger(true, t.Name())
	client := l1connection.NewClient(l1.Config, log)

	initialOutputCount := mustOutputCount(client, myAddress)
	//
	// Check if faucet requests are working.
	require.NoError(t, client.RequestFunds(myKeyPair))
	for i := 0; ; i++ {
		t.Log("Waiting for a TX...")
		time.Sleep(100 * time.Millisecond)
		if mustOutputCount(client, myAddress) > initialOutputCount {
			break
		}
	}

	//
	// Check if the TX post works.
	kp2 := cryptolib.NewKeyPair()
	addr2 := kp2.GetPublicKey().AsEd25519Address()
	initialOutputCountAddr2 := mustOutputCount(client, addr2)

	blockIssuerID, err := util.BlockIssuerAccountIDFromOutputs(mustOutputMap(client, myAddress))
	require.NoError(t, err)

	block := makeSimpleValueTX(t, client, myKeyPair, addr2, 500_000, blockIssuerID)
	err = client.PostBlockAndWaitUntilConfirmation(block)
	require.NoError(t, err)

	for i := 0; ; i++ {
		t.Log("Waiting for a TX...")
		time.Sleep(100 * time.Millisecond)
		if initialOutputCountAddr2 != mustOutputCount(client, addr2) {
			break
		}
	}
}

func makeSimpleValueTX(
	t *testing.T,
	c l1connection.Client,
	sender cryptolib.VariantKeyPair,
	recipientAddr iotago.Address,
	amount iotago.BaseToken,
	blockIssuerID iotago.AccountID,
) *iotago.Block {
	senderAddr := sender.Address()
	senderAccountOutputs, err := c.OutputMap(senderAddr)
	require.NoError(t, err)

	l1API := c.APIProvider().LatestAPI()
	txBuilder := builder.NewTransactionBuilder(l1API, sender)

	// use the funds in the account output
	accountUTXOID, accountUTXO := util.AccountOutputFromOutputs(senderAccountOutputs)
	require.NotNil(t, accountUTXO)
	minSD, err := l1API.StorageScoreStructure().MinDeposit(accountUTXO)
	require.NoError(t, err)
	// cannot send everything (must have at least minSD to keep the accoutn output alive)
	require.GreaterOrEqual(t, accountUTXO.Amount-minSD, amount)

	// add the account UTXO as input
	txBuilder.AddInput(&builder.TxInput{
		UnlockTarget: senderAddr,
		InputID:      accountUTXOID,
		Input:        accountUTXO,
	})

	// add the target utxo
	txBuilder.AddOutput(&iotago.BasicOutput{
		Amount:           amount,
		UnlockConditions: iotago.BasicOutputUnlockConditions{&iotago.AddressUnlockCondition{Address: recipientAddr}},
	})

	// create the new output for the account
	newAccountUTXO := accountUTXO.Clone().(*iotago.AccountOutput)
	newAccountUTXO.Amount -= amount // update the amount in the account output
	newAccountUTXO.Mana = 0         // set mana to 0, it will be updated later
	txBuilder.AddOutput(newAccountUTXO)

	blockIssuance, err := c.APIProvider().BlockIssuance(context.Background())
	require.NoError(t, err)

	block, err := transaction.FinalizeTxAndBuildBlock(
		l1API,
		txBuilder,
		blockIssuance,
		1,
		blockIssuerID,
		sender,
	)
	require.NoError(t, err)
	return block
}

func mustOutputCount(client l1connection.Client, myAddress iotago.Address) int {
	return len(mustOutputMap(client, myAddress))
}

func mustOutputMap(client l1connection.Client, myAddress iotago.Address) map[iotago.OutputID]iotago.Output {
	outs, err := client.OutputMap(myAddress)
	if err != nil {
		panic(fmt.Errorf("unable to get outputs as a map: %w", err))
	}
	return outs
}
