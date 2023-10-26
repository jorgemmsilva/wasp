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
	accountOutput, err := ch.LatestAccountOutput(chaintypes.ActiveOrCommittedState)
	if err != nil {
		return nil, fmt.Errorf("could not get latest AccountOutput: %w", err)
	}
	blockTime := isc.BlockTime{
		SlotIndex: accountOutput.OutputID().CreationSlot() + 1,
		Timestamp: time.Now(),
	}
	res, err := runISCRequest(ch, accountOutput, blockTime, req, estimateGas)
	if err != nil {
		return nil, err
	}
	return res.Receipt, nil
}
