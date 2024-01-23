package webapi

import (
	"time"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/wasp/packages/authentication"
	"github.com/iotaledger/wasp/packages/webapi"
)

type ParametersWebAPI struct {
	Enabled                   bool                             `default:"true" usage:"whether the web api plugin is enabled"`
	BindAddress               string                           `default:"0.0.0.0:9090" usage:"the bind address for the node web api"`
	Auth                      authentication.AuthConfiguration `usage:"configures the authentication for the API service"`
	IndexDbPath               string                           `default:"waspdb/chains/index" usage:"directory for storing indexes of historical data (only archive nodes will create/use them)"`
	Limits                    webapi.ParametersWebAPILimits
	DebugRequestLoggerEnabled bool `default:"false" usage:"whether the debug logging for requests should be enabled"`
}

var ParamsWebAPI = &ParametersWebAPI{
	Auth: authentication.AuthConfiguration{
		Scheme: "jwt",
		JWTConfig: authentication.JWTAuthConfiguration{
			Duration: 24 * time.Hour,
		},
	},
}

var params = &app.ComponentParams{
	Params: map[string]any{
		"webapi": ParamsWebAPI,
	},
	Masked: nil,
}
