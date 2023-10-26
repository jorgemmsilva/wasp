package solo

import (
	"testing"

	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/vm/core/corecontracts"
)

func TestSoloBasic1(t *testing.T) {
	parameters.InitL1(parameters.L1ForTesting)
	corecontracts.PrintWellKnownHnames()
	env := New(t, &InitOptions{Debug: true, PrintStackTrace: true})
	_ = env.NewChain()
}
