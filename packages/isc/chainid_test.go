package isc_test

import (
	"testing"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

func TestChainIDSerialization(t *testing.T) {
	chainID := isc.RandomChainID()
	rwutil.ReadWriteTest(t, &chainID, new(isc.ChainID))
	rwutil.BytesTest(t, chainID, isc.ChainIDFromBytes)
	rwutil.Bech32Test(t, chainID, isc.ChainIDFromBech32, "test")
}
