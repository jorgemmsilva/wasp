package corecontracts

import (
	"net/http"

	"github.com/labstack/echo/v4"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/webapi/corecontracts"
	"github.com/iotaledger/wasp/packages/webapi/models"
	"github.com/iotaledger/wasp/packages/webapi/params"
)

func MapGovChainInfoResponse(chainInfo *isc.ChainInfo, l1API iotago.API) models.GovChainInfoResponse {
	return models.GovChainInfoResponse{
		ChainID:      chainInfo.ChainID.Bech32(l1API.ProtocolParameters().Bech32HRP()),
		ChainOwnerID: chainInfo.ChainOwnerID.Bech32(l1API.ProtocolParameters().Bech32HRP()),
		GasFeePolicy: chainInfo.GasFeePolicy,
		GasLimits:    chainInfo.GasLimits,
		PublicURL:    chainInfo.PublicURL,
		Metadata: models.GovPublicChainMetadata{
			EVMJsonRPCURL:   chainInfo.Metadata.EVMJsonRPCURL,
			EVMWebSocketURL: chainInfo.Metadata.EVMWebSocketURL,
			Name:            chainInfo.Metadata.Name,
			Description:     chainInfo.Metadata.Description,
			Website:         chainInfo.Metadata.Website,
		},
	}
}

func (c *Controller) getChainInfo(e echo.Context) error {
	invoker, ch, err := c.createCallViewInvoker(e)
	if err != nil {
		return err
	}

	chainInfo, err := corecontracts.GetChainInfo(invoker, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	chainInfoResponse := MapGovChainInfoResponse(chainInfo, c.l1Api)

	return e.JSON(http.StatusOK, chainInfoResponse)
}

func (c *Controller) getChainOwner(e echo.Context) error {
	invoker, ch, err := c.createCallViewInvoker(e)
	if err != nil {
		return err
	}

	chainOwner, err := corecontracts.GetChainOwner(invoker, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	chainOwnerResponse := models.GovChainOwnerResponse{
		ChainOwner: chainOwner.Bech32(c.l1Api.ProtocolParameters().Bech32HRP()),
	}

	return e.JSON(http.StatusOK, chainOwnerResponse)
}

func (c *Controller) getAllowedStateControllerAddresses(e echo.Context) error {
	invoker, ch, err := c.createCallViewInvoker(e)
	if err != nil {
		return err
	}

	addresses, err := corecontracts.GetAllowedStateControllerAddresses(invoker, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	encodedAddresses := make([]string, len(addresses))

	for k, v := range addresses {
		encodedAddresses[k] = v.Bech32(c.l1Api.ProtocolParameters().Bech32HRP())
	}

	addressesResponse := models.GovAllowedStateControllerAddressesResponse{
		Addresses: encodedAddresses,
	}

	return e.JSON(http.StatusOK, addressesResponse)
}
