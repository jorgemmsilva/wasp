package isc

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/tpkg"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

func TestAgentIDSerialization(t *testing.T) {
	prefix := iotago.NetworkPrefix("test")

	n := &NilAgentID{}
	rwutil.BytesTest(t, AgentID(n), AgentIDFromBytes)
	rwutil.Bech32Test(t, AgentID(n), AgentIDFromBech32, prefix)

	a := NewAddressAgentID(tpkg.RandEd25519Address())
	rwutil.BytesTest(t, AgentID(a), AgentIDFromBytes)
	rwutil.Bech32Test(t, AgentID(a), AgentIDFromBech32, prefix)
	rwutil.Bech32Test(t, a, addressAgentIDFromString, prefix)

	chainID := ChainIDFromAddress(tpkg.RandAnchorAddress())
	c := NewContractAgentID(chainID, 42)
	rwutil.BytesTest(t, AgentID(c), AgentIDFromBytes)
	rwutil.Bech32Test(t, AgentID(c), AgentIDFromBech32, prefix)

	e := NewEthereumAddressAgentID(chainID, common.HexToAddress("1074"))
	rwutil.BytesTest(t, AgentID(e), AgentIDFromBytes)
	rwutil.Bech32Test(t, AgentID(e), AgentIDFromBech32, prefix)
	rwutil.Bech32Test(t, AgentID(e), AgentIDFromBech32, prefix)
}
