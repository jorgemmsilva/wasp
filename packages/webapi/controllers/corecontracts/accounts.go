package corecontracts

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/iota.go/v4/hexutil"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/webapi/apierrors"
	"github.com/iotaledger/wasp/packages/webapi/controllers/controllerutils"
	"github.com/iotaledger/wasp/packages/webapi/corecontracts"
	"github.com/iotaledger/wasp/packages/webapi/models"
	"github.com/iotaledger/wasp/packages/webapi/params"
)

func (c *Controller) getAccounts(e echo.Context) error {
	ch, chainID, err := controllerutils.ChainFromParams(e, c.chainService)
	if err != nil {
		return c.handleViewCallError(err, chainID)
	}

	invoker := corecontracts.MakeCallViewInvoker(ch, c.l1Api, c.baseTokenInfo)
	accounts, err := corecontracts.GetAccounts(invoker, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, chainID)
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
	ch, chainID, err := controllerutils.ChainFromParams(e, c.chainService)
	if err != nil {
		return c.handleViewCallError(err, chainID)
	}

	invoker := corecontracts.MakeCallViewInvoker(ch, c.l1Api, c.baseTokenInfo)
	assets, err := corecontracts.GetTotalAssets(invoker, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, chainID)
	}

	assetsResponse := &models.FungibleTokensResponse{
		BaseTokens:   hexutil.EncodeUint64(uint64(assets.BaseTokens)),
		NativeTokens: isc.NativeTokenMapToJSONObject(assets.NativeTokens),
	}

	return e.JSON(http.StatusOK, assetsResponse)
}

func (c *Controller) getAccountBalance(e echo.Context) error {
	ch, chainID, err := controllerutils.ChainFromParams(e, c.chainService)
	if err != nil {
		return c.handleViewCallError(err, chainID)
	}

	agentID, err := params.DecodeAgentID(e)
	if err != nil {
		return err
	}

	invoker := corecontracts.MakeCallViewInvoker(ch, c.l1Api, c.baseTokenInfo)
	assets, err := corecontracts.GetAccountBalance(invoker, agentID, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, chainID)
	}

	assetsResponse := &models.FungibleTokensResponse{
		BaseTokens:   hexutil.EncodeUint64(uint64(assets.BaseTokens)),
		NativeTokens: isc.NativeTokenMapToJSONObject(assets.NativeTokens),
	}

	return e.JSON(http.StatusOK, assetsResponse)
}

func (c *Controller) getAccountNFTs(e echo.Context) error {
	ch, chainID, err := controllerutils.ChainFromParams(e, c.chainService)
	if err != nil {
		return c.handleViewCallError(err, chainID)
	}

	agentID, err := params.DecodeAgentID(e)
	if err != nil {
		return err
	}

	invoker := corecontracts.MakeCallViewInvoker(ch, c.l1Api, c.baseTokenInfo)
	nfts, err := corecontracts.GetAccountNFTs(invoker, agentID, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, chainID)
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
	ch, chainID, err := controllerutils.ChainFromParams(e, c.chainService)
	if err != nil {
		return c.handleViewCallError(err, chainID)
	}
	agentID, err := params.DecodeAgentID(e)
	if err != nil {
		return err
	}

	invoker := corecontracts.MakeCallViewInvoker(ch, c.l1Api, c.baseTokenInfo)
	foundries, err := corecontracts.GetAccountFoundries(invoker, agentID, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, chainID)
	}

	return e.JSON(http.StatusOK, &models.AccountFoundriesResponse{
		FoundrySerialNumbers: foundries,
	})
}

func (c *Controller) getAccountNonce(e echo.Context) error {
	ch, chainID, err := controllerutils.ChainFromParams(e, c.chainService)
	if err != nil {
		return c.handleViewCallError(err, chainID)
	}

	agentID, err := params.DecodeAgentID(e)
	if err != nil {
		return err
	}

	invoker := corecontracts.MakeCallViewInvoker(ch, c.l1Api, c.baseTokenInfo)
	nonce, err := corecontracts.GetAccountNonce(invoker, agentID, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, chainID)
	}

	nonceResponse := &models.AccountNonceResponse{
		Nonce: hexutil.EncodeUint64(nonce),
	}

	return e.JSON(http.StatusOK, nonceResponse)
}

func (c *Controller) getNFTData(e echo.Context) error {
	ch, chainID, err := controllerutils.ChainFromParams(e, c.chainService)
	if err != nil {
		return c.handleViewCallError(err, chainID)
	}

	nftID, err := params.DecodeNFTID(e)
	if err != nil {
		return err
	}

	invoker := corecontracts.MakeCallViewInvoker(ch, c.l1Api, c.baseTokenInfo)
	nftData, err := corecontracts.GetNFTData(invoker, *nftID, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, chainID)
	}

	nftDataResponse := isc.NFTToJSONObject(nftData, c.l1Api)

	return e.JSON(http.StatusOK, nftDataResponse)
}

func (c *Controller) getNativeTokenIDRegistry(e echo.Context) error {
	ch, chainID, err := controllerutils.ChainFromParams(e, c.chainService)
	if err != nil {
		return c.handleViewCallError(err, chainID)
	}

	invoker := corecontracts.MakeCallViewInvoker(ch, c.l1Api, c.baseTokenInfo)
	registries, err := corecontracts.GetNativeTokenIDRegistry(invoker, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, chainID)
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
	ch, chainID, err := controllerutils.ChainFromParams(e, c.chainService)
	if err != nil {
		return c.handleViewCallError(err, chainID)
	}

	serialNumber, err := params.DecodeUInt(e, "serialNumber")
	if err != nil {
		return err
	}

	invoker := corecontracts.MakeCallViewInvoker(ch, c.l1Api, c.baseTokenInfo)
	foundryOutput, err := corecontracts.GetFoundryOutput(invoker, c.l1Api, uint32(serialNumber), e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, chainID)
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
