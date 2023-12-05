package isc

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/tpkg"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

func bech32Test[T interface {
	Bech32(prefix iotago.NetworkPrefix) string
}](
	t *testing.T,
	obj1 T,
	fromString func(prefix iotago.NetworkPrefix, data string) (T, error),
) T {
	const prefix = "tst"

	obj2, err := fromString(prefix, obj1.Bech32(prefix))
	require.NoError(t, err)
	require.Equal(t, obj1, obj2)
	require.Equal(t, obj1.Bech32(prefix), obj2.Bech32(prefix))
	return obj2
}

func TestAgentIDSerialization(t *testing.T) {
	n := &NilAgentID{}
	rwutil.BytesTest(t, AgentID(n), AgentIDFromBytes)
	bech32Test(t, AgentID(n), AgentIDFromString)

	a := NewAddressAgentID(tpkg.RandEd25519Address())
	rwutil.BytesTest(t, AgentID(a), AgentIDFromBytes)
	bech32Test(t, AgentID(a), AgentIDFromString)
	bech32Test(t, a, addressAgentIDFromString)

	chainID := ChainIDFromAddress(tpkg.RandAnchorAddress())
	c := NewContractAgentID(chainID, 42)
	rwutil.BytesTest(t, AgentID(c), AgentIDFromBytes)
	bech32Test(t, AgentID(c), AgentIDFromString)

	e := NewEthereumAddressAgentID(chainID, common.HexToAddress("1074"))
	rwutil.BytesTest(t, AgentID(e), AgentIDFromBytes)
	bech32Test(t, AgentID(e), AgentIDFromString)
}
