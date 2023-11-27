package chainutil

import (
	"time"

	"github.com/ethereum/go-ethereum/eth/tracers"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/isc"
)

func EVMTraceTransaction(
	ch chaintypes.ChainCore,
	chainOutputs *isc.ChainOutputs,
	timestamp time.Time,
	iscRequestsInBlock []isc.Request,
	txIndex uint64,
	tracer tracers.Tracer,
	l1API iotago.API,
	tokenInfo api.InfoResBaseToken,
) error {
	_, err := runISCTask(
		ch,
		chainOutputs,
		timestamp,
		iscRequestsInBlock,
		false,
		&isc.EVMTracer{
			Tracer:  tracer,
			TxIndex: txIndex,
		},
		l1API,
		tokenInfo,
	)
	return err
}
