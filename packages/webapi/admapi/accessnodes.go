package admapi

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/webapi/httperrors"
	"github.com/iotaledger/wasp/packages/webapi/routes"

	"github.com/labstack/echo/v4"
	"github.com/pangpanglabs/echoswagger/v2"
	"github.com/samber/lo"
)

func addAccessNodesEndpoints(
	adm echoswagger.ApiGroup,
	registryProvider registry.Provider,
	tnm peering.TrustedNetworkManager,
) {
	a := &accessNodesService{
		registry:   registryProvider,
		networkMgr: tnm,
	}
	adm.POST(routes.AdmAddAccessNode(":chainID"), a.handleAddAccessNode).
		AddParamPath("", "chainID", "ChainID (string)").
		AddParamPath("", "nodeID", "NodeID (string)").
		SetSummary("Add an access node to a chain")

	adm.POST(routes.AdmRemoveAccessNode(":chainID"), a.handleRemoveAccessNode).
		AddParamPath("", "chainID", "ChainID (string)").
		SetSummary("Remove an access node from a chain")
}

type accessNodesService struct {
	registry   registry.Provider
	networkMgr peering.TrustedNetworkManager
}

func (a *accessNodesService) chainRecordFromParams(c echo.Context) (*registry.ChainRecord, error) {
	chainID, err := isc.ChainIDFromString(c.Param("chainID"))
	if err != nil {
		return nil, httperrors.BadRequest("invalid chainID")
	}
	chainRec, err := a.registry().GetChainRecordByChainID(chainID)
	if err != nil {
		return nil, httperrors.NotFound("chain not found")
	}
	return chainRec, nil
}

func (a *accessNodesService) handleAddAccessNode(c echo.Context) error {
	chainRec, err := a.chainRecordFromParams(c)
	if err != nil {
		return err
	}
	nodeID := c.Param("nodeID")
	peers, err := a.networkMgr.TrustedPeers()
	if err != nil {
		return httperrors.ServerError("error getting trusted peers")
	}
	_, ok := lo.Find(peers, func(p *peering.TrustedPeer) bool {
		return p.NetID == nodeID
	})
	if !ok {
		return httperrors.NotFound(fmt.Sprintf("couldn't find peer with netID %s", nodeID))
	}
	err = chainRec.AddAccessNode(nodeID)
	if err != nil {
		return httperrors.ServerError(fmt.Sprintf("error adding access node. %s", err.Error()))
	}
	err = a.registry().SaveChainRecord(chainRec)
	if err != nil {
		return httperrors.ServerError("error saving chain record.")
	}
	return c.NoContent(http.StatusOK)
}

func (a *accessNodesService) handleRemoveAccessNode(c echo.Context) error {
	chainRec, err := a.chainRecordFromParams(c)
	if err != nil {
		return err
	}
	nodeID := c.Param("nodeID")
	err = chainRec.RemoveAccessNode(nodeID)
	if err != nil {
		return httperrors.ServerError(fmt.Sprintf("error removing access node. %s", err.Error()))
	}
	err = a.registry().SaveChainRecord(chainRec)
	if err != nil {
		return httperrors.ServerError("error saving chain record.")
	}
	return c.NoContent(http.StatusOK)
}
