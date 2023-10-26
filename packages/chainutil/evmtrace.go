package chainutil

import (
	"github.com/ethereum/go-ethereum/eth/tracers"

	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/isc"
)

func EVMTraceTransaction(
	ch chaintypes.ChainCore,
	accountOutput *isc.AccountOutputWithID,
	blockTime isc.BlockTime,
	iscRequestsInBlock []isc.Request,
	txIndex uint64,
	tracer tracers.Tracer,
) error {
	_, err := runISCTask(
		ch,
		accountOutput,
		blockTime,
		iscRequestsInBlock,
		false,
		&isc.EVMTracer{
			Tracer:  tracer,
			TxIndex: txIndex,
		},
	)
	return err
}
