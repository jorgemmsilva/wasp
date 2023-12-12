package chainutil

import (
	"fmt"
	"time"

	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
)

func SimulateRequest(
	ch chaintypes.ChainCore,
	req isc.Request,
	estimateGas bool,
) (*blocklog.RequestReceipt, error) {
	chainOutputs, err := ch.LatestChainOutputs(chaintypes.ActiveOrCommittedState)
	if err != nil {
		return nil, fmt.Errorf("could not get latest AnchorOutput: %w", err)
	}
	res, err := runISCRequest(ch, chainOutputs, time.Now(), req, estimateGas)
	if err != nil {
		return nil, err
	}
	return res.Receipt, nil
}
