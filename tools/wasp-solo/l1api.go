package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/iota.go/v4/nodeclient"
	"github.com/iotaledger/wasp/components/app"
	"github.com/iotaledger/wasp/packages/solo"
)

func response(env *solo.Solo, c echo.Context, obj any) error {
	if c.Request().Header.Get(echo.HeaderAccept) == api.MIMEApplicationVendorIOTASerializerV2 {
		b, err := env.L1APIProvider().LatestAPI().Encode(obj)
		if err != nil {
			return err
		}
		c.Blob(http.StatusOK, api.MIMEApplicationVendorIOTASerializerV2, b)
	} else {
		json, err := env.L1APIProvider().LatestAPI().JSONEncode(obj)
		if err != nil {
			return err
		}
		c.JSONBlob(http.StatusOK, json)
	}
	return nil
}

func errorHandler(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		if err != nil {
			status := c.Response().Status
			var httpError *echo.HTTPError
			if errors.As(err, &httpError) {
				status = httpError.Code
			}
			if status == 0 || status == http.StatusOK {
				status = http.StatusInternalServerError
			}
			return c.JSON(status, &nodeclient.HTTPErrorResponseEnvelope{
				Error: struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				}{
					Code:    strconv.Itoa(status),
					Message: err.Error(),
				},
			})
		}
		return nil
	}
}

func l1ApiInit(env *solo.Solo, e *echo.Echo) {
	g := e.Group("/l1api")

	g.Use(errorHandler)

	g.GET("/api/core/v3/info", func(c echo.Context) error {
		return response(env, c, &api.InfoResponse{
			Name:    "l1api-mock",
			Version: app.Version,
			Status: &api.InfoResNodeStatus{
				IsHealthy:                   true,
				AcceptedTangleTime:          time.Now(),
				RelativeAcceptedTangleTime:  time.Now(),
				ConfirmedTangleTime:         time.Now(),
				RelativeConfirmedTangleTime: time.Now(),
				LatestCommitmentID:          [36]byte{},
				LatestFinalizedSlot:         env.SlotIndex(),
				LatestAcceptedBlockSlot:     env.SlotIndex(),
				LatestConfirmedBlockSlot:    env.SlotIndex(),
				PruningEpoch:                0,
			},
			Metrics: &api.InfoResNodeMetrics{
				BlocksPerSecond:          0,
				ConfirmedBlocksPerSecond: 0,
				ConfirmationRate:         0,
			},
			ProtocolParameters: []*api.InfoResProtocolParameters{{
				StartEpoch: 0,
				Parameters: env.L1APIProvider().APIForEpoch(0).ProtocolParameters(),
			}},
			BaseToken: env.TokenInfo(),
		})
	})

	g.GET("/api/routes", func(c echo.Context) error {
		return response(env, c, &api.RoutesResponse{
			Routes: []iotago.PrefixedStringUint8{
				api.CorePluginName,
				api.IndexerPluginName,
			},
		})
	})

	outputKinds := map[string]struct {
		Type       iotago.OutputType
		QueryParam string
	}{
		"basic":   {Type: iotago.OutputBasic, QueryParam: "address"},
		"account": {Type: iotago.OutputAccount, QueryParam: "address"},
		"anchor":  {Type: iotago.OutputAnchor, QueryParam: "governor"},
		"foundry": {Type: iotago.OutputFoundry, QueryParam: "accountAddress"},
		"nft":     {Type: iotago.OutputNFT, QueryParam: "address"},
	}
	g.GET("/api/indexer/v2/outputs/:kind", func(c echo.Context) error {
		kind := outputKinds[c.Param("kind")]
		netPrefix, addr, err := iotago.ParseBech32(c.QueryParam(kind.QueryParam))
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		netPrefixExpected := env.L1APIProvider().LatestAPI().ProtocolParameters().Bech32HRP()
		if netPrefix != netPrefixExpected {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("expected network prefix %q, got: %q",
				netPrefixExpected,
				netPrefix,
			))
		}
		outs := env.L1Ledger().GetUnspentOutputs(addr)
		println(outs, len(outs))
		filteredOuts := lo.Filter(lo.Entries(outs), func(e lo.Entry[iotago.OutputID, iotago.Output], index int) bool {
			return e.Value.Type() == kind.Type
		})
		return response(env, c, &api.IndexerResponse{
			CommittedSlot: env.SlotIndex(),
			PageSize:      uint32(len(outs)),
			Items: lo.Map(filteredOuts, func(e lo.Entry[iotago.OutputID, iotago.Output], index int) iotago.HexOutputID {
				return iotago.HexOutputID(e.Key.ToHex())
			}),
		})
	})

	g.GET("/api/core/v3/outputs/:id", func(c echo.Context) error {
		id, err := iotago.OutputIDFromHexString(c.Param("id"))
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		out := env.L1Ledger().GetOutput(id)
		if out == nil {
			return echo.ErrNotFound
		}
		tx, ok := env.L1Ledger().GetTransaction(id.TransactionID())
		if !ok {
			return echo.ErrNotFound
		}
		proof, err := iotago.OutputIDProofFromTransaction(tx.Transaction, id.Index())
		if err != nil {
			return err
		}
		return response(env, c, &api.OutputResponse{
			Output:        out,
			OutputIDProof: proof,
		})
	})

	postedBlocks := make(map[iotago.BlockID]*iotago.SignedTransaction)

	g.POST("/api/core/v3/blocks", func(c echo.Context) error {
		body, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return err
		}
		var block iotago.Block
		_, err = env.L1APIProvider().LatestAPI().Decode(body, &block)
		if err != nil {
			return err
		}
		switch block.Body.Type() {
		case iotago.BlockBodyTypeBasic:
		default:
			return fmt.Errorf("No handler for block body type %d", block.Body.Type())
		}
		blockBody := block.Body.(*iotago.BasicBlockBody)
		switch blockBody.Payload.PayloadType() {
		case iotago.PayloadSignedTransaction:
		default:
			return fmt.Errorf("No handler for payload type %d", blockBody.Payload.PayloadType())
		}
		tx := blockBody.Payload.(*iotago.SignedTransaction)

		blockID, err := block.ID()
		if err != nil {
			return err
		}
		if postedBlocks[blockID] != nil {
			return echo.ErrConflict
		}

		err = env.AddToLedger(tx)
		if err != nil {
			return err
		}
		postedBlocks[blockID] = tx

		c.Response().Header().Set("Location", blockID.ToHex())
		return nil
	})

	g.GET("/api/core/v3/blocks/:id/metadata", func(c echo.Context) error {
		blockID, err := iotago.BlockIDFromHexString(c.Param("id"))
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		tx := postedBlocks[blockID]
		if tx == nil {
			return echo.ErrNotFound
		}
		txID, err := tx.Transaction.ID()
		if err != nil {
			return err
		}
		return response(env, c, &api.BlockMetadataResponse{
			BlockID:    blockID,
			BlockState: api.BlockStateConfirmed,
			TransactionMetadata: &api.TransactionMetadataResponse{
				TransactionID:    txID,
				TransactionState: api.TransactionStateConfirmed,
			},
		})
	})
}

func l1FaucetInit(env *solo.Solo, e *echo.Echo) {
	g := e.Group("/l1faucet")

	g.Use(errorHandler)

	g.POST("/api/enqueue", func(c echo.Context) error {
		body, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return err
		}
		var req struct {
			Address string `json:"address"`
		}
		err = json.Unmarshal(body, &req)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		prefix, addr, err := iotago.ParseBech32(req.Address)
		if prefix != env.L1APIProvider().CommittedAPI().ProtocolParameters().Bech32HRP() {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid network prefix")
		}
		_, err = env.L1Ledger().GetFundsFromFaucet(addr)
		if err != nil {
			return err
		}
		c.NoContent(http.StatusAccepted)
		return nil
	})
}
