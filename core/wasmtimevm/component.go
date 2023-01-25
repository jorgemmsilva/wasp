// Wasp can have several VM types. Each of them can be represented by separate plugin
// Plugin name serves as a VM type during dynamic loading of the binary.
// VM plugins can be enabled/disabled in the configuration of the node instance
// wasmtimevm plugin statically links VM implemented with Wasmtime to Wasp
// be registering wasmhost.GetProcessor as function
package wasmtimevm

import (
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/app"
	"github.com/iotaledger/wasp/packages/vm/processors"
	"github.com/iotaledger/wasp/packages/vm/vmtypes"
)

func init() {
	CoreComponent = &app.CoreComponent{
		Component: &app.Component{
			Name:      "WasmTimeVM",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Configure: configure,
		},
	}
}

var (
	CoreComponent *app.CoreComponent
	deps          dependencies
)

type dependencies struct {
	dig.In

	ProcessorsConfig *processors.Config
}

func configure() error {
	// register VM type(s)
	// TODO (via config?) pass non-default timeout for WasmTime processor like this:
	// WasmTimeout = 3 * time.Second
	_, err := deps.ProcessorsConfig.WithWasmVM(CoreComponent.Logger())
	if err != nil {
		CoreComponent.LogPanic(err)
	}
	CoreComponent.LogInfof("registered VM type: '%s'", vmtypes.WasmTime)

	return nil
}
