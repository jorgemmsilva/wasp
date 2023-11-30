package corecontracts

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/iota.go/v4/hexutil"
	"github.com/iotaledger/wasp/packages/webapi/corecontracts"
	"github.com/iotaledger/wasp/packages/webapi/params"
)

type Blob struct {
	Hash string `json:"hash" swagger:"required"`
	Size uint32 `json:"size" swagger:"required,min(1)"`
}

type BlobListResponse struct {
	Blobs []Blob
}

func (c *Controller) listBlobs(e echo.Context) error {
	invoker, ch, err := c.createCallViewInvoker(e)
	if err != nil {
		return err
	}

	blobList, err := corecontracts.ListBlobs(invoker, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	blobListResponse := &BlobListResponse{
		Blobs: make([]Blob, 0, len(blobList)),
	}

	for k, v := range blobList {
		blobListResponse.Blobs = append(blobListResponse.Blobs, Blob{
			Hash: k.Hex(),
			Size: v,
		})
	}

	return e.JSON(http.StatusOK, blobListResponse)
}

type BlobValueResponse struct {
	ValueData string `json:"valueData" swagger:"required,desc(The blob data (Hex))"`
}

func (c *Controller) getBlobValue(e echo.Context) error {
	invoker, ch, err := c.createCallViewInvoker(e)
	if err != nil {
		return err
	}

	blobHash, err := params.DecodeBlobHash(e)
	if err != nil {
		return err
	}

	fieldKey := e.Param(params.ParamFieldKey)

	blobValueBytes, err := corecontracts.GetBlobValue(invoker, *blobHash, fieldKey, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	blobValueResponse := &BlobValueResponse{
		ValueData: hexutil.EncodeHex(blobValueBytes),
	}

	return e.JSON(http.StatusOK, blobValueResponse)
}

type BlobInfoResponse struct {
	Fields map[string]uint32 `json:"fields" swagger:"required,min(1)"`
}

func (c *Controller) getBlobInfo(e echo.Context) error {
	invoker, ch, err := c.createCallViewInvoker(e)
	if err != nil {
		return err
	}

	blobHash, err := params.DecodeBlobHash(e)
	if err != nil {
		return err
	}

	blobInfo, ok, err := corecontracts.GetBlobInfo(invoker, *blobHash, e.QueryParam(params.ParamBlockIndexOrTrieRoot))
	if err != nil {
		return c.handleViewCallError(err, ch.ID())
	}

	blobInfoResponse := &BlobInfoResponse{
		Fields: map[string]uint32{},
	}

	// TODO: Validate this logic. Should an empty blob info result in a different http error code?
	if !ok {
		fmt.Printf("GET BLOB INFO return empty blobInfoResponse")
		return e.JSON(http.StatusOK, blobInfoResponse)
	}

	for k, v := range blobInfo {
		blobInfoResponse.Fields[k] = v
	}

	return e.JSON(http.StatusOK, blobInfoResponse)
}
