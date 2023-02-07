package corecontracts

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/webapi/v2/apierrors"
	"github.com/iotaledger/wasp/packages/webapi/v2/params"
)

type gasFeePolicy struct {
	GasFeeTokenID     string `json:"gasFeeTokenId" swagger:"desc(The gas fee token id. Empty if base token.),required"`
	GasPerToken       uint64 `json:"gasPerToken" swagger:"desc(The amount of gas per token.),required"`
	ValidatorFeeShare uint8  `json:"validatorFeeShare" swagger:"desc(The validator fee share.),required"`
}

type GovChainInfoResponse struct {
	ChainID         string       `json:"chainID" swagger:"desc(ChainID (Bech32-encoded).),required"`
	ChainOwnerID    string       `json:"chainOwnerId" swagger:"desc(The chain owner address (Bech32-encoded).),required"`
	Description     string       `json:"description" swagger:"desc(The description of the chain.),required"`
	GasFeePolicy    gasFeePolicy `json:"gasFeePolicy" swagger:"desc(The gas fee policy),required"`
	MaxBlobSize     uint32       `json:"maxBlobSize" swagger:"desc(The maximum contract blob size.),required"`
	MaxEventSize    uint16       `json:"maxEventSize" swagger:"desc(The maximum event size.),required"`                      // TODO: Clarify
	MaxEventsPerReq uint16       `json:"maxEventsPerReq" swagger:"desc(The maximum amount of events per request.),required"` // TODO: Clarify
}

func MapGovChainInfoResponse(chainInfo *governance.ChainInfo) GovChainInfoResponse {
	gasFeeTokenID := ""

	feePolicy, err := chainInfo.GasFeePolicyDeserialized()
	if err != nil {
		panic(err)
	}

	chainID, err := chainInfo.ChainIDDeserialized()
	if err != nil {
		panic(err)
	}

	chOwner, err := chainInfo.ChainOwnerIDDeserialized()
	if err != nil {
		panic(err)
	}

	if !isc.IsEmptyNativeTokenID(feePolicy.GasFeeTokenID) {
		gasFeeTokenID = feePolicy.GasFeeTokenID.String()
	}

	chainInfoResponse := GovChainInfoResponse{
		ChainID:      chainID.String(),
		ChainOwnerID: chOwner.String(),
		Description:  chainInfo.Description,
		GasFeePolicy: gasFeePolicy{
			GasFeeTokenID:     gasFeeTokenID,
			GasPerToken:       feePolicy.GasPerToken,
			ValidatorFeeShare: feePolicy.ValidatorFeeShare,
		},
		MaxBlobSize:     chainInfo.MaxBlobSize,
		MaxEventSize:    chainInfo.MaxEventSize,
		MaxEventsPerReq: chainInfo.MaxEventsPerReq,
	}

	return chainInfoResponse
}

func (c *Controller) getChainInfo(e echo.Context) error {
	chainID, err := params.DecodeChainID(e)
	if err != nil {
		return err
	}

	chainInfo, err := c.governance.GetChainInfo(chainID)
	if err != nil {
		return apierrors.ContractExecutionError(err)
	}

	chainInfoResponse := MapGovChainInfoResponse(chainInfo)

	return e.JSON(http.StatusOK, chainInfoResponse)
}
