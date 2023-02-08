package dashboard

import (
	_ "embed"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/wasp/packages/chain"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/blob"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/evm"
)

//go:embed templates/chain.tmpl
var tplChain string

func chainBreadcrumb(e *echo.Echo, chainID isc.ChainID) Tab {
	return Tab{
		Path:  e.Reverse("chain"),
		Title: fmt.Sprintf("Chain %.8s…", chainID.String()),
		Href:  e.Reverse("chain", chainID.String()),
	}
}

func (d *Dashboard) initChain(e *echo.Echo, r renderer) {
	route := e.GET("/chain/:chainid", d.handleChain)
	route.Name = "chain"
	r[route.Path] = d.makeTemplate(e, tplChain)
}

func (d *Dashboard) handleChain(c echo.Context) error {
	chainID, err := isc.ChainIDFromString(c.Param("chainid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	tab := chainBreadcrumb(c.Echo(), chainID)

	result := &ChainTemplateParams{
		BaseTemplateParams: d.BaseParams(c, tab),
		ChainID:            chainID,
	}

	result.Record, err = d.wasp.GetChainRecord(chainID)
	if err != nil {
		return err
	}

	if result.Record != nil && result.Record.Active {
		result.LatestBlock, err = d.getLatestBlock(chainID)
		if err != nil {
			return err
		}

		result.Committee, err = d.wasp.GetChainCommitteeInfo(chainID)
		if err != nil {
			return err
		}

		result.ChainInfo, err = d.fetchChainInfo(chainID)
		if err != nil {
			return err
		}

		result.Accounts, err = d.fetchAccounts(chainID)
		if err != nil {
			return err
		}

		result.TotalAssets, err = d.fetchTotalAssets(chainID)
		if err != nil {
			return err
		}

		result.Blobs, err = d.fetchBlobs(chainID)
		if err != nil {
			return err
		}

		result.EVMChainID, err = d.fetchEVMChainID(chainID)
		if err != nil {
			return err
		}
	}

	return c.Render(http.StatusOK, c.Path(), result)
}

func (d *Dashboard) getLatestBlock(chainID isc.ChainID) (*blocklog.BlockInfo, error) {
	ret, err := d.wasp.CallView(chainID, blocklog.Contract.Name, blocklog.ViewGetBlockInfo.Name, nil)
	if err != nil {
		return nil, err
	}
	bInfo, err := util.Deserialize[*blocklog.BlockInfo](ret)
	if err != nil {
		return nil, err
	}
	return bInfo, nil
}

func (d *Dashboard) fetchAccounts(chainID isc.ChainID) ([]isc.AgentID, error) {
	data, err := d.wasp.CallView(chainID, accounts.Contract.Name, accounts.ViewAccounts.Name, nil)
	if err != nil {
		return nil, err
	}
	accs, err := dict.FromBytes(data)
	if err != nil {
		return nil, err
	}
	ret := make([]isc.AgentID, 0)
	for k := range accs {
		agentid, err := codec.DecodeAgentID([]byte(k))
		if err != nil {
			return nil, err
		}
		ret = append(ret, agentid)
	}
	return ret, nil
}

func (d *Dashboard) fetchTotalAssets(chainID isc.ChainID) (*isc.Assets, error) {
	data, err := d.wasp.CallView(chainID, accounts.Contract.Name, accounts.ViewTotalAssets.Name, nil)
	if err != nil {
		return nil, err
	}
	bal, err := dict.FromBytes(data)
	if err != nil {
		return nil, err
	}
	return isc.AssetsFromDict(bal)
}

func (d *Dashboard) fetchBlobs(chainID isc.ChainID) (map[hashing.HashValue]uint32, error) {
	data, err := d.wasp.CallView(chainID, blob.Contract.Name, blob.ViewListBlobs.Name, nil)
	if err != nil {
		return nil, err
	}
	ret, err := util.Deserialize[map[hashing.HashValue]uint32](data)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (d *Dashboard) fetchEVMChainID(chainID isc.ChainID) (uint16, error) {
	data, err := d.wasp.CallView(chainID, evm.Contract.Name, evm.FuncGetChainID.Name, nil)
	if err != nil {
		return 0, err
	}
	return util.Deserialize[uint16](data)
}

type ChainTemplateParams struct {
	BaseTemplateParams

	ChainID isc.ChainID

	EVMChainID  uint16
	Record      *registry.ChainRecord
	LatestBlock *blocklog.BlockInfo
	ChainInfo   *ChainInfo
	Accounts    []isc.AgentID
	TotalAssets *isc.Assets
	Blobs       map[hashing.HashValue]uint32
	Committee   *chain.CommitteeInfo
}
