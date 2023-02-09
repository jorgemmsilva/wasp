package scclient

import (
	"github.com/iotaledger/wasp/packages/kv/dict"
)

func (c *SCClient) CallView(functionName string, args dict.Dict) ([]byte, error) {
	return c.ChainClient.CallView(c.ContractHname, functionName, args)
}
