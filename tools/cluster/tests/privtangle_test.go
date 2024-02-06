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
	myAddress := myKeyPair.GetPublicKey().AsEd25519Address()

	nc, err := nodeclient.New(l1.Config.APIAddress)
	require.NoError(t, err)
	_, err = nc.Info(ctx)
	require.NoError(t, err)

	log := testlogger.NewSilentLogger(true, t.Name())
	client := l1connection.NewClient(l1.Config, log)

	initialOutputCount := mustOutputCount(client, myAddress)
	//
	// Check if faucet requests are working.
	client.RequestFunds(myAddress)
	for i := 0; ; i++ {
		t.Log("Waiting for a TX...")
		time.Sleep(100 * time.Millisecond)
		if initialOutputCount != mustOutputCount(client, myAddress) {
			break
		}
	}

	// TODO needs to be fixed, tx post using using the issuer is not working as expected
	// //
	// // Check if the TX post works.
	// kp2 := cryptolib.NewKeyPair()
	// addr2 := kp2.GetPublicKey().AsEd25519Address()
	// initialOutputCount = mustOutputCount(client, addr2)
	// tx, err := l1connection.MakeSimpleValueTX(client, myKeyPair, addr2, 500_000)
	// require.NoError(t, err)
	// _, err = client.PostTxAndWaitUntilConfirmation(tx)
	// require.NoError(t, err)
	//
	//	for i := 0; ; i++ {
	//		t.Log("Waiting for a TX...")
	//		time.Sleep(100 * time.Millisecond)
	//		if initialOutputCount != mustOutputCount(client, addr2) {
	//			break
	//		}
	//	}
}

func mustOutputCount(client l1connection.Client, myAddress *iotago.Ed25519Address) int {
	return len(mustOutputMap(client, myAddress))
}

func mustOutputMap(client l1connection.Client, myAddress *iotago.Ed25519Address) map[iotago.OutputID]iotago.Output {
	outs, err := client.OutputMap(myAddress)
	if err != nil {
		panic(fmt.Errorf("unable to get outputs as a map: %w", err))
	}
	return outs
}
