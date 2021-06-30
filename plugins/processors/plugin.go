package processors

import (
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/wasp/contracts/native/inccounter"
	"github.com/iotaledger/wasp/packages/coretypes/coreutil"
	"github.com/iotaledger/wasp/packages/vm/processors"
)

const pluginName = "Processors"

var (
	log    *logger.Logger
	Config *processors.Config
)

func Init() *node.Plugin {
	return node.NewPlugin(pluginName, node.Enabled, configure, run)
}

func configure(ctx *node.Plugin) {
	log = logger.NewLogger(pluginName)

	log.Info("Registering native contracts...")
	nativeContracts := []*coreutil.ContractInterface{
		inccounter.Interface,
	}
	for _, c := range nativeContracts {
		log.Debugf(
			"Registering native contract: name: '%s', program hash: %s, description: '%s'\n",
			c.Name, c.ProgramHash.String(), c.Description,
		)
	}
	Config = processors.NewConfig(nativeContracts...)
}

func run(_ *node.Plugin) {
}
