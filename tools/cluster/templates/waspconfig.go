package templates

type WaspConfigParams struct {
	APIPort       int
	DashboardPort int
	PeeringPort   int
	NanomsgPort   int
	Neighbors     string
}

const WaspConfig = `
{
  "database": {
    "inMemory": true,
    "directory": "waspdb"
  },
  "logger": {
    "level": "info",
    "disableCaller": false,
    "disableStacktrace": true,
    "encoding": "console",
    "outputPaths": [
      "stdout",
      "wasp.log"
    ],
    "disableEvents": true
  },
  "network": {
    "bindAddress": "0.0.0.0",
    "externalAddress": "auto"
  },
  "node": {
    "disablePlugins": [],
    "enablePlugins": []
  },
  "webapi": {
    "bindAddress": "0.0.0.0:{{.ApiPort}}"
  },
  "dashboard": {
    "bindAddress": "0.0.0.0:{{.DashboardPort}}"
  },
  "peering":{
    "port": {{.PeeringPort}},
    "netid": "127.0.0.1:{{.PeeringPort}}",
	"neighbors": [{{.Neighbors}}]
  },
  "nodeconn": {
    "address": "127.0.0.1:5000"
  },
  "nanomsg":{
    "port": {{.NanomsgPort}}
  }
}
`
