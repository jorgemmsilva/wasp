package processors

import (
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/app"
	"github.com/iotaledger/wasp/packages/vm/core/coreprocessors"
	"github.com/iotaledger/wasp/packages/vm/processors"
)

func init() {
	CoreComponent = &app.CoreComponent{
		Component: &app.Component{
			Name:    "Processors",
			Provide: provide,
		},
	}
}

var CoreComponent *app.CoreComponent

func provide(c *dig.Container) error {
	type processorsConfigResult struct {
		dig.Out

		ProcessorsConfig *processors.Config
	}

	if err := c.Provide(func() processorsConfigResult {
		CoreComponent.LogInfo("Registering native contracts...")
		return processorsConfigResult{
			ProcessorsConfig: coreprocessors.NewConfigWithCoreContracts(),
		}
	}); err != nil {
		CoreComponent.LogPanic(err)
	}

	return nil
}
