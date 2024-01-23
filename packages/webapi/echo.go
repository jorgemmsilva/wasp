package webapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pangpanglabs/echoswagger/v2"

	"github.com/iotaledger/hive.go/log"
	"github.com/iotaledger/inx-app/pkg/httpserver"
	"github.com/iotaledger/wasp/packages/authentication"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/metrics"
	"github.com/iotaledger/wasp/packages/webapi/apierrors"
	"github.com/iotaledger/wasp/packages/webapi/controllers/controllerutils"
)

type ParametersWebAPILimits struct {
	Timeout                        time.Duration `default:"30s" usage:"the timeout after which a long running operation will be canceled"`
	ReadTimeout                    time.Duration `default:"10s" usage:"the read timeout for the HTTP request body"`
	WriteTimeout                   time.Duration `default:"60s" usage:"the write timeout for the HTTP response body"`
	MaxBodyLength                  string        `default:"2M" usage:"the maximum number of characters that the body of an API call may contain"`
	MaxTopicSubscriptionsPerClient int           `default:"0" usage:"defines the max amount of subscriptions per client. 0 = deactivated (default)"`
	ConfirmedStateLagThreshold     uint32        `default:"2" usage:"the threshold that define a chain is unsynchronized"`
	Jsonrpc                        ParametersJSONRPC
}

type ParametersJSONRPC struct {
	MaxBlocksInLogsFilterRange int `default:"1000" usage:"maximum amount of blocks in eth_getLogs filter range"`
	MaxLogsInResult            int `default:"10000" usage:"maximum amount of logs in eth_getLogs result"`

	WebsocketRateLimitMessagesPerSecond int           `default:"20" usage:"the websocket rate limit (messages per second)"`
	WebsocketRateLimitBurst             int           `default:"5" usage:"the websocket burst limit"`
	WebsocketConnectionCleanupDuration  time.Duration `default:"5m" usage:"defines in which interval stale connections will be cleaned up"`
	WebsocketClientBlockDuration        time.Duration `default:"5m" usage:"the duration a misbehaving client will be blocked"`
}

//nolint:funlen
func NewEcho(
	debug bool,
	limits *ParametersWebAPILimits,
	jwtDuration time.Duration,
	metrics *metrics.ChainMetricsProvider,
	version string,
	log log.Logger,
) echoswagger.ApiRoot {
	e := httpserver.NewEcho(log, nil, debug)

	e.Server.ReadTimeout = limits.ReadTimeout
	e.Server.WriteTimeout = limits.WriteTimeout

	e.HidePort = true
	e.HTTPErrorHandler = apierrors.HTTPErrorHandler()

	ConfirmedStateLagThreshold = limits.ConfirmedStateLagThreshold
	authentication.DefaultJWTDuration = jwtDuration

	e.Pre(middleware.RemoveTrailingSlash())

	if metrics != nil {
		// publish metrics to prometheus component (that exposes a separate http server on another port)
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				if strings.HasPrefix(c.Path(), "/chains/") {
					// ignore metrics for all requests not related to "chains/<chainID>""
					return next(c)
				}
				start := time.Now()
				err := next(c)

				status := c.Response().Status
				if err != nil {
					var httpError *echo.HTTPError
					if errors.As(err, &httpError) {
						status = httpError.Code
					}
					if status == 0 || status == http.StatusOK {
						status = http.StatusInternalServerError
					}
				}

				chainID, ok := c.Get(controllerutils.EchoContextKeyChainID).(isc.ChainID)
				if !ok {
					return err
				}

				operation, ok := c.Get(controllerutils.EchoContextKeyOperation).(string)
				if !ok {
					return err
				}
				metrics.GetChainMetrics(chainID).WebAPI.WebAPIRequest(operation, status, time.Since(start))
				return err
			}
		})
	}

	// timeout middleware
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			timeoutCtx, cancel := context.WithTimeout(c.Request().Context(), limits.Timeout)
			defer cancel()

			c.SetRequest(c.Request().WithContext(timeoutCtx))

			return next(c)
		}
	})

	e.Use(middlewareUnescapePath)

	e.Use(middleware.BodyLimit(limits.MaxBodyLength))

	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: `${time_rfc3339_nano} ${remote_ip} ${method} ${uri} ${status} error="${error}"` + "\n",
	}))

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowHeaders:     []string{"*"},
		AllowMethods:     []string{"*"},
		AllowCredentials: true,
	}))

	swagger := CreateEchoSwagger(e, version)

	if debug {
		swagger.Echo().Use(middleware.BodyDump(func(c echo.Context, reqBody, resBody []byte) {
			log.LogDebugf("API Dump: Request=%q, Response=%q", reqBody, resBody)
		}))
	}

	return swagger
}

func CreateEchoSwagger(e *echo.Echo, version string) echoswagger.ApiRoot {
	echoSwagger := echoswagger.New(e, "/doc", &echoswagger.Info{
		Title:       "Wasp API",
		Description: "REST API for the Wasp node",
		Version:     version,
	})

	echoSwagger.AddSecurityAPIKey("Authorization", "JWT Token", echoswagger.SecurityInHeader).
		SetExternalDocs("Find out more about Wasp", "https://wiki.iota.org/smart-contracts/overview").
		SetUI(echoswagger.UISetting{DetachSpec: false, HideTop: false}).
		SetScheme("http", "https")

	echoSwagger.SetRequestContentType(echo.MIMEApplicationJSON)
	echoSwagger.SetResponseContentType(echo.MIMEApplicationJSON)

	return echoSwagger
}
