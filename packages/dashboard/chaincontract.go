package dashboard

import (
	_ "embed"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/root"
)

//go:embed templates/chaincontract.tmpl
var tplChainContract string

func (d *Dashboard) initChainContract(e *echo.Echo, r renderer) {
	route := e.GET("/chain/:chainid/contract/:hname", d.handleChainContract)
	route.Name = "chainContract"
	r[route.Path] = d.makeTemplate(e, tplChainContract)
}

func (d *Dashboard) handleChainContract(c echo.Context) error {
	chainID, err := isc.ChainIDFromString(c.Param("chainid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("chainid: %w", err))
	}

	hname, err := isc.HnameFromHexString(c.Param("hname"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("hname: %w", err))
	}

	result := &ChainContractTemplateParams{
		BaseTemplateParams: d.BaseParams(c, chainBreadcrumb(c.Echo(), chainID), Tab{
			Path:  c.Path(),
			Title: fmt.Sprintf("Contract %d", hname),
			Href:  "#",
		}),
		ChainID: chainID,
		Hname:   hname,
	}

	contractBytes, err := d.wasp.CallView(chainID, root.Contract.Name, root.ViewFindContract.Name, codec.MakeDict(map[string]interface{}{
		root.ParamHname: codec.EncodeHname(hname),
	}))
	if err != nil {
		return fmt.Errorf("call view failed: %w", err)
	}

	result.ContractRecord, err = root.ContractRecordFromBytes(contractBytes)
	if err != nil {
		return fmt.Errorf("cannot decode contract record: %w", err)
	}

	data, err := d.wasp.CallView(chainID, blocklog.Contract.Name, blocklog.ViewGetEventsForContract.Name, codec.MakeDict(map[string]interface{}{
		blocklog.ParamContractHname: codec.EncodeHname(hname),
	}))
	if err != nil {
		return fmt.Errorf("call view failed: %w", err)
	}
	result.Log, err = util.Deserialize[[]string](data)
	if err != nil {
		return fmt.Errorf("deserializing call view result failed: %w", err)
	}

	return c.Render(http.StatusOK, c.Path(), result)
}

type ChainContractTemplateParams struct {
	BaseTemplateParams

	ChainID isc.ChainID
	Hname   isc.Hname

	ContractRecord *root.ContractRecord
	Log            []string
}
