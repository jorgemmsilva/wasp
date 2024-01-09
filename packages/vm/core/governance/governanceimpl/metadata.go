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
	if publicURLOpt != nil {
		ctx.Requiref(
			len(*publicURLOpt) <= transaction.MaxPublicURLLength,
			"supplied publicUrl is too long (%d>%d)", len(*publicURLOpt), transaction.MaxPublicURLLength,
		)
		governance.SetPublicURL(ctx.State(), *publicURLOpt)
	}
	if metadataOpt != nil {
		governance.SetMetadata(ctx.State(), *metadataOpt)
	}
	return nil
}

func getMetadata(ctx isc.SandboxView) (string, *isc.PublicChainMetadata) {
	publicURL, _ := governance.GetPublicURL(ctx.StateR())
	metadata := lo.Must(governance.GetMetadata(ctx.StateR()))
	return publicURL, metadata
}
