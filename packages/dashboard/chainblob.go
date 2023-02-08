package dashboard

import (
	_ "embed"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/blob"
)

//go:embed templates/chainblob.tmpl
var tplChainBlob string

func (d *Dashboard) initChainBlob(e *echo.Echo, r renderer) {
	route := e.GET("/chain/:chainid/blob/:hash", d.handleChainBlob)
	route.Name = "chainBlob"
	r[route.Path] = d.makeTemplate(e, tplChainBlob)

	route = e.GET("/chain/:chainid/blob/:hash/raw/:field", d.handleChainBlobDownload)
	route.Name = "chainBlobDownload"
}

func (d *Dashboard) handleChainBlob(c echo.Context) error {
	chainID, err := isc.ChainIDFromString(c.Param("chainid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	hash, err := hashing.HashValueFromHex(c.Param("hash"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	result := &ChainBlobTemplateParams{
		BaseTemplateParams: d.BaseParams(c, chainBreadcrumb(c.Echo(), chainID), Tab{
			Path:  c.Path(),
			Title: fmt.Sprintf("Blob %.8s…", c.Param("hash")),
			Href:  "#",
		}),
		ChainID: chainID,
		Hash:    hash,
	}

	data, err := d.wasp.CallView(chainID, blob.Contract.Name, blob.ViewGetBlobInfo.Name, codec.MakeDict(map[string]interface{}{
		blob.ParamHash: hash,
	}))
	if err != nil {
		return err
	}
	fields, err := util.Deserialize[map[string]uint32](data)
	if err != nil {
		return err
	}
	result.Blob = make([]BlobField, len(fields))
	i := 0
	for key := range fields {
		value, err := d.wasp.CallView(chainID, blob.Contract.Name, blob.ViewGetBlobField.Name, codec.MakeDict(map[string]interface{}{
			blob.ParamHash:  hash,
			blob.ParamField: []byte(key),
		}))
		if err != nil {
			return err
		}
		result.Blob[i] = BlobField{
			Key:   []byte(key),
			Value: value,
		}
		i++
	}
	return c.Render(http.StatusOK, c.Path(), result)
}

func (d *Dashboard) handleChainBlobDownload(c echo.Context) error {
	chainID, err := isc.ChainIDFromString(c.Param("chainid"))
	if err != nil {
		return err
	}

	hash, err := hashing.HashValueFromHex(c.Param("hash"))
	if err != nil {
		return err
	}

	field, err := iotago.DecodeHex(c.Param("field"))
	if err != nil {
		return err
	}

	value, err := d.wasp.CallView(chainID, blob.Contract.Name, blob.ViewGetBlobField.Name, codec.MakeDict(map[string]interface{}{
		blob.ParamHash:  hash,
		blob.ParamField: field,
	}))
	if err != nil {
		return err
	}

	return c.Blob(http.StatusOK, "application/octet-stream", value)
}

type ChainBlobTemplateParams struct {
	BaseTemplateParams

	ChainID isc.ChainID
	Hash    hashing.HashValue

	Blob []BlobField
}

type BlobField struct {
	Key   []byte
	Value []byte
}
