// needed to link packages with examples and to force link some packages
package globals

import (
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/wasp/packages/vm/viewcontext"
)

// PluginName is the name of the banner plugin.
const PluginName = "Globals"

func Init() *node.Plugin {
	return node.NewPlugin(PluginName, node.Enabled, configure, run)
}

func configure(_ *node.Plugin) {
	viewcontext.InitLogger()
}

func run(_ *node.Plugin) {
}
