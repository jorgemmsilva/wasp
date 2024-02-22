package governanceimpl

import (
	"github.com/samber/lo"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
)

func setMetadata(ctx isc.Sandbox, publicURLOpt *string, metadataOpt **isc.PublicChainMetadata) dict.Dict {
	ctx.RequireCallerIsChainOwner()
	state := governance.NewStateWriterFromSandbox(ctx)
	if publicURLOpt != nil {
		ctx.Requiref(
			len(*publicURLOpt) <= transaction.MaxPublicURLLength,
			"supplied publicUrl is too long (%d>%d)", len(*publicURLOpt), transaction.MaxPublicURLLength,
		)
		state.SetPublicURL(*publicURLOpt)
	}
	if metadataOpt != nil {
		state.SetMetadata(*metadataOpt)
	}
	return nil
}

func getMetadata(ctx isc.SandboxView) (string, *isc.PublicChainMetadata) {
	state := governance.NewStateReaderFromSandbox(ctx)
	publicURL, _ := state.GetPublicURL()
	metadata := lo.Must(state.GetMetadata())
	return publicURL, metadata
}
