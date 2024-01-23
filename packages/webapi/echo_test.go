package webapi_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/log"

	"github.com/iotaledger/wasp/packages/webapi"
)

func TestInternalServerErrors(t *testing.T) {
	// start a webserver with a test log
	logOutput := bytes.NewBuffer(nil)
	log := log.NewLogger(
		log.WithOutput(logOutput),
	)

	e := webapi.NewEcho(
		true,
		&webapi.ParametersWebAPILimits{
			Timeout:                        time.Minute,
			ReadTimeout:                    time.Minute,
			WriteTimeout:                   time.Minute,
			MaxBodyLength:                  "1M",
			MaxTopicSubscriptionsPerClient: 0,
			ConfirmedStateLagThreshold:     2,
			Jsonrpc:                        webapi.ParametersJSONRPC{},
		},
		24*time.Hour,
		nil,
		"",
		log,
	)

	// Add an endpoint that just panics with "foobar" and start the server
	exceptionText := "foobar"
	e.GET("/test", func(c echo.Context) error { panic(exceptionText) })
	go func() {
		err := e.Echo().Start(":9999")
		require.ErrorIs(t, http.ErrServerClosed, err)
	}()
	defer e.Echo().Shutdown(context.Background())

	// query the endpoint
	req, err := http.NewRequest(http.MethodGet, "http://localhost:9999/test", http.NoBody)
	require.NoError(t, err)

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	resBody, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	res.Body.Close()

	// assert the exception is not present in the response (prevent leaking errors)
	require.Equal(t, res.StatusCode, http.StatusInternalServerError)
	require.NotContains(t, string(resBody), exceptionText)

	// assert the exception is logged
	logEntries := bytes.Split(logOutput.Bytes(), []byte("\n"))
	require.Len(t, logEntries, 2) // "" after last newline
	require.Contains(t, string(logEntries[0]), exceptionText)
}
