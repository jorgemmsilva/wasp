package chainutil

import (
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/vm/core/errors"
)

func ResolveError(ch chaintypes.ChainCore, e *isc.UnresolvedVMError) (*isc.VMError, error) {
	s, err := ch.LatestState(chaintypes.ActiveOrCommittedState)
	if err != nil {
		return nil, err
	}
	return errors.ResolveFromState(s, e)
}
