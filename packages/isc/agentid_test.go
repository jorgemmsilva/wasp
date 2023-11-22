package isc_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/tpkg"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

func TestAgentIDSerialization(t *testing.T) {
	n := &isc.NilAgentID{}
	rwutil.BytesTest(t, isc.AgentID(n), isc.AgentIDFromBytes)
	testStrFromBech32[isc.AgentID](t, isc.AgentID(n), isc.AgentIDFromString, testutil.L1API.ProtocolParameters().Bech32HRP())

	a := isc.NewAddressAgentID(tpkg.RandEd25519Address())
	rwutil.BytesTest(t, isc.AgentID(a), isc.AgentIDFromBytes)
	testStrFromBech32[isc.AgentID](t, isc.AgentID(a), isc.AgentIDFromString, testutil.L1API.ProtocolParameters().Bech32HRP())

	chainID := isc.ChainIDFromAddress(tpkg.RandAnchorAddress())
	c := isc.NewContractAgentID(chainID, 42)
	rwutil.BytesTest(t, isc.AgentID(c), isc.AgentIDFromBytes)
	testStrFromBech32[isc.AgentID](t, isc.AgentID(c), isc.AgentIDFromString, testutil.L1API.ProtocolParameters().Bech32HRP())

	e := isc.NewEthereumAddressAgentID(chainID, common.HexToAddress("1074"))
	rwutil.BytesTest(t, isc.AgentID(e), isc.AgentIDFromBytes)
	testStrFromBech32[isc.AgentID](t, isc.AgentID(e), isc.AgentIDFromString, testutil.L1API.ProtocolParameters().Bech32HRP())
}

func testStrFromBech32[T interface {
	String(iotago.NetworkPrefix) string
}](t *testing.T, obj1 T, fromString func(data string, bech32HRP iotago.NetworkPrefix) (T, error), bech32HRP iotago.NetworkPrefix,
) T {
	obj2, err := fromString(obj1.String(bech32HRP), bech32HRP)
	require.NoError(t, err)
	require.Equal(t, obj1, obj2)
	require.Equal(t, obj1.String(bech32HRP), obj2.String(bech32HRP))
	return obj2
}
