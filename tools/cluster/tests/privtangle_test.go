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
	"github.com/iotaledger/iota.go/v4/nodeclient"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/l1connection"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
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

	blockIssuerID, err := util.BlockIssuerFromOutputs(mustOutputMap(client, myAddress))
	require.NoError(t, err)

	tx, err := client.MakeSimpleValueTX(myKeyPair, addr2, 500_000, blockIssuerID)
	require.NoError(t, err)

	_, err = client.PostTxAndWaitUntilConfirmation(tx, blockIssuerID, myKeyPair)
	require.NoError(t, err)

	for i := 0; ; i++ {
		t.Log("Waiting for a TX...")
		time.Sleep(100 * time.Millisecond)
		if initialOutputCountAddr2 != mustOutputCount(client, addr2) {
			break
		}
	}
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
