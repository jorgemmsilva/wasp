package chainclient

import (
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/dict"
)

// CallView sends a request to call a view function of a given contract, and returns the result of the call
func (c *Client) CallView(hContract isc.Hname, functionName string, args dict.Dict) ([]byte, error) {
	return c.WaspClient.CallView(c.ChainID, hContract, functionName, args)
}
