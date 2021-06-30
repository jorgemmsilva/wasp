package committee

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/wasp/packages/coretypes"
	"github.com/iotaledger/wasp/packages/registry/committee_record"
	"github.com/iotaledger/wasp/packages/tcrypto"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
	"github.com/iotaledger/wasp/packages/testutil/testpeers"
	"github.com/stretchr/testify/require"
)

func TestCommitteeBasic(t *testing.T) {
	suite := tcrypto.DefaultSuite()
	log := testlogger.NewLogger(t)
	defer log.Sync()
	nodeCount := 4
	netIDs, identities := testpeers.SetupKeys(uint16(nodeCount))
	stateAddr, dksRegistries := testpeers.SetupDkgPregenerated(t, uint16((len(netIDs)*2)/3+1), netIDs, suite)
	nodes := testpeers.SetupNet(netIDs, identities, testutil.NewPeeringNetReliable(), log)
	net0 := nodes[0]

	cfg0 := &committeeimplTestConfigProvider{
		ownNetID:  netIDs[0],
		neighbors: netIDs,
	}

	cmtRec := &committee_record.CommitteeRecord{
		Address: stateAddr,
		Nodes:   netIDs,
	}
	c, err := New(cmtRec, nil, net0, cfg0, dksRegistries[0], log)
	require.NoError(t, err)
	require.True(t, c.Address().Equals(stateAddr))
	require.EqualValues(t, 4, c.Size())
	require.EqualValues(t, 3, c.Quorum())

	time.Sleep(100 * time.Millisecond)
	require.True(t, c.IsReady())
	c.Close()
	require.False(t, c.IsReady())
}

var _ coretypes.PeerNetworkConfigProvider = &committeeimplTestConfigProvider{}

// TODO: should this object be obtained from peering.NetworkProvider?
// Or should coretypes.PeerNetworkConfigProvider methods methods be part of
// peering.NetworkProvider interface
type committeeimplTestConfigProvider struct {
	ownNetID  string
	neighbors []string
}

func (p *committeeimplTestConfigProvider) OwnNetID() string {
	return p.ownNetID
}

func (p *committeeimplTestConfigProvider) PeeringPort() int {
	return 0 // Anything
}

func (p *committeeimplTestConfigProvider) Neighbors() []string {
	return p.neighbors
}

func (p *committeeimplTestConfigProvider) String() string {
	return fmt.Sprintf("committeeimplPeerConfig( ownNetID: %s, neighbors: %+v )", p.OwnNetID(), p.Neighbors())
}
