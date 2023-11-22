package isc_test

import (
	"testing"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/util/rwutil"
	"github.com/stretchr/testify/require"
)

func TestChainIDSerialization(t *testing.T) {
	chainID := isc.RandomChainID()
	rwutil.ReadWriteTest(t, &chainID, new(isc.ChainID))
	rwutil.BytesTest(t, chainID, isc.ChainIDFromBytes)

	chainID2, err := isc.ChainIDFromString(chainID.String(testutil.L1API.ProtocolParameters().Bech32HRP()))
	require.NoError(t, err)
	require.Equal(t, chainID, chainID2)
	require.Equal(t, chainID.String(testutil.L1API.ProtocolParameters().Bech32HRP()), chainID2.String(testutil.L1API.ProtocolParameters().Bech32HRP()))
}
