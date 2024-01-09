package corecontracts

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/iota.go/v4/hexutil"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/webapi/apierrors"
	"github.com/iotaledger/wasp/packages/webapi/corecontracts"
	"github.com/iotaledger/wasp/packages/webapi/models"
	"github.com/iotaledger/wasp/packages/webapi/params"
)

func (c *Controller) getAccounts(e echo.Context) error {
	invoker, ch, err := c.createCallViewInvoker(e)
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	accounts, err := corecontracts.GetAccounts(invoker, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	accountsResponse := &models.AccountListResponse{
		Accounts: make([]string, len(accounts)),
	}

	for k, v := range accounts {
		accountsResponse.Accounts[k] = v.String()
	}

	return e.JSON(http.StatusOK, accountsResponse)
}

func (c *Controller) getTotalAssets(e echo.Context) error {
	invoker, ch, err := c.createCallViewInvoker(e)
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	assets, err := corecontracts.GetTotalAssets(invoker, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	assetsResponse := &models.FungibleTokensResponse{
		BaseTokens:   hexutil.EncodeUint64(uint64(assets.BaseTokens)),
		NativeTokens: isc.NativeTokenMapToJSONObject(assets.NativeTokens),
	}

	return e.JSON(http.StatusOK, assetsResponse)
}

func (c *Controller) getAccountBalance(e echo.Context) error {
	invoker, ch, err := c.createCallViewInvoker(e)
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	agentID, err := params.DecodeAgentID(e)
	if err != nil {
		return err
	}

	assets, err := corecontracts.GetAccountBalance(invoker, agentID, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	assetsResponse := &models.FungibleTokensResponse{
		BaseTokens:   hexutil.EncodeUint64(uint64(assets.BaseTokens)),
		NativeTokens: isc.NativeTokenMapToJSONObject(assets.NativeTokens),
	}

	return e.JSON(http.StatusOK, assetsResponse)
}

func (c *Controller) getAccountNFTs(e echo.Context) error {
	invoker, ch, err := c.createCallViewInvoker(e)
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	agentID, err := params.DecodeAgentID(e)
	if err != nil {
		return err
	}

	nfts, err := corecontracts.GetAccountNFTs(invoker, agentID, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	nftsResponse := &models.AccountNFTsResponse{
		NFTIDs: make([]string, len(nfts)),
	}

	for k, v := range nfts {
		nftsResponse.NFTIDs[k] = v.ToHex()
	}

	return e.JSON(http.StatusOK, nftsResponse)
}

func (c *Controller) getAccountFoundries(e echo.Context) error {
	invoker, ch, err := c.createCallViewInvoker(e)
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	agentID, err := params.DecodeAgentID(e)
	if err != nil {
		return err
	}

	foundries, err := corecontracts.GetAccountFoundries(invoker, agentID, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	return e.JSON(http.StatusOK, &models.AccountFoundriesResponse{
		FoundrySerialNumbers: foundries,
	})
}

func (c *Controller) getAccountNonce(e echo.Context) error {
	invoker, ch, err := c.createCallViewInvoker(e)
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	agentID, err := params.DecodeAgentID(e)
	if err != nil {
		return err
	}

	nonce, err := corecontracts.GetAccountNonce(invoker, agentID, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	nonceResponse := &models.AccountNonceResponse{
		Nonce: hexutil.EncodeUint64(nonce),
	}

	return e.JSON(http.StatusOK, nonceResponse)
}

func (c *Controller) getNFTData(e echo.Context) error {
	invoker, ch, err := c.createCallViewInvoker(e)
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	nftID, err := params.DecodeNFTID(e)
	if err != nil {
		return err
	}

	nftData, err := corecontracts.GetNFTData(invoker, *nftID, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	nftDataResponse := isc.NFTToJSONObject(nftData, c.l1Api)

	return e.JSON(http.StatusOK, nftDataResponse)
}

func (c *Controller) getNativeTokenIDRegistry(e echo.Context) error {
	invoker, ch, err := c.createCallViewInvoker(e)
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	registries, err := corecontracts.GetNativeTokenIDRegistry(invoker, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	nativeTokenIDRegistryResponse := &models.NativeTokenIDRegistryResponse{
		NativeTokenRegistryIDs: make([]string, len(registries)),
	}

	for k, v := range registries {
		nativeTokenIDRegistryResponse.NativeTokenRegistryIDs[k] = v.String()
	}

	return e.JSON(http.StatusOK, nativeTokenIDRegistryResponse)
}

func (c *Controller) getFoundryOutput(e echo.Context) error {
	invoker, ch, err := c.createCallViewInvoker(e)
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	serialNumber, err := params.DecodeUInt(e, "serialNumber")
	if err != nil {
		return err
	}

	foundryOutput, err := corecontracts.GetFoundryOutput(invoker, c.l1Api, uint32(serialNumber), e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	foundryOutputID, err := foundryOutput.FoundryID()
	if err != nil {
		return apierrors.InvalidPropertyError("FoundryOutput.ID", err)
	}

	nativeTokenID, err := foundryOutput.NativeTokenID()

	foundryOutputResponse := &models.FoundryOutputResponse{
		FoundryID:     foundryOutputID.ToHex(),
		BaseTokens:    hexutil.EncodeUint64(uint64(foundryOutput.Amount)),
		NativeTokenID: nativeTokenID.String(),
	}

	return e.JSON(http.StatusOK, foundryOutputResponse)
}
