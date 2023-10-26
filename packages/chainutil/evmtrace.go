package chainutil

import (
	"time"

	"github.com/ethereum/go-ethereum/eth/tracers"

	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/isc"
)

func EVMTraceTransaction(
	ch chaintypes.ChainCore,
	accountOutput *isc.AccountOutputWithID,
	timestamp time.Time,
	iscRequestsInBlock []isc.Request,
	txIndex uint64,
	tracer tracers.Tracer,
) error {
	_, err := runISCTask(
		ch,
		accountOutput,
		timestamp,
		iscRequestsInBlock,
		false,
		&isc.EVMTracer{
			Tracer:  tracer,
			TxIndex: txIndex,
		},
	)
	return err
}
