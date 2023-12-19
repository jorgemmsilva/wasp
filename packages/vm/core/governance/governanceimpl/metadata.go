package governanceimpl

import (
	"github.com/samber/lo"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
)

func setMetadata(ctx isc.Sandbox) dict.Dict {
	ctx.RequireCallerIsChainOwner()

	var publicURLBytes []byte
	var metadataBytes []byte

	publicURLBytes = ctx.Params().Get(governance.ParamPublicURL)
	ctx.Requiref(len(publicURLBytes) <= transaction.MaxPublicURLLength, "supplied publicUrl is too long (%d>%d)", len(publicURLBytes), transaction.MaxPublicURLLength)
	metadataBytes = ctx.Params().Get(governance.ParamMetadata)

	if publicURLBytes != nil {
		publicURL, err := codec.DecodeString(publicURLBytes, "")
		ctx.RequireNoError(err)
		governance.SetPublicURL(ctx.State(), publicURL)
	}

	if metadataBytes != nil {
		metadata, err := isc.PublicChainMetadataFromBytes(metadataBytes)
		ctx.RequireNoError(err)
		governance.SetMetadata(ctx.State(), metadata)
	}

	return nil
}

func getMetadata(ctx isc.SandboxView) dict.Dict {
	publicURL, _ := governance.GetPublicURL(ctx.StateR())
	metadata := lo.Must(governance.GetMetadata(ctx.StateR()))

	return dict.Dict{
		governance.ParamPublicURL: []byte(publicURL),
		governance.ParamMetadata:  metadata.Bytes(),
	}
}
