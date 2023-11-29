package l1starter_test

import (
	"context"
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/nodeclient"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/l1connection"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
	"github.com/iotaledger/wasp/packages/util/l1starter"
)

// TODO remove this file, it is tested by privtangle_test.go already..
// just testing here because the other test dir won't compile for now

func TestPrivtangleTMP(t *testing.T) {
	l1 := l1starter.New(flag.CommandLine, flag.CommandLine)
	l1.StartPrivtangleIfNecessary(t.Logf)
	t.Cleanup(func() { l1.Cleanup(t.Logf) })

	ctx := context.Background()

	//
	// Try call the faucet.
	myKeyPair := cryptolib.NewKeyPair()
	myAddress := myKeyPair.GetPublicKey().AsEd25519Address()

	nc, err := nodeclient.New(l1.Config.APIAddress)
	require.NoError(t, err)
	_, err = nc.Info(ctx)
	require.NoError(t, err)

	log := testlogger.NewSilentLogger(t.Name(), true)
	client := l1connection.NewClient(l1.Config, log)

	initialOutputCount := mustOutputCount(client, myAddress)
	//
	// Check if faucet requests are working.
	err = client.RequestFunds(myAddress)
	require.NoError(t, err)
	for i := 0; ; i++ {
		t.Log("Waiting for a TX...")
		time.Sleep(100 * time.Millisecond)
		if initialOutputCount != mustOutputCount(client, myAddress) {
			break
		}
	}
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
