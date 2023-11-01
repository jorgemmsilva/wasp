package chainutil

import (
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/vmimpl"
)

func runISCTask(
	ch chaintypes.ChainCore,
	chainOutputs *isc.ChainOutputs,
	timestamp time.Time,
	reqs []isc.Request,
	estimateGasMode bool,
	evmTracer *isc.EVMTracer,
) ([]*vm.RequestResult, error) {
	task := &vm.VMTask{
		Processors:           ch.Processors(),
		Inputs:               chainOutputs,
		Store:                ch.Store(),
		Requests:             reqs,
		Timestamp:            timestamp,
		Entropy:              hashing.PseudoRandomHash(nil),
		ValidatorFeeTarget:   accounts.CommonAccount(),
		EnableGasBurnLogging: estimateGasMode,
		EstimateGasMode:      estimateGasMode,
		EVMTracer:            evmTracer,
		Log:                  ch.Log().Desugar().WithOptions(zap.AddCallerSkip(1)).Sugar(),
	}
	res, err := vmimpl.Run(task)
	if err != nil {
		return nil, err
	}
	return res.RequestResults, nil
}

func runISCRequest(
	ch chaintypes.ChainCore,
	chainOutputs *isc.ChainOutputs,
	timestamp time.Time,
	req isc.Request,
	estimateGasMode bool,
) (*vm.RequestResult, error) {
	results, err := runISCTask(
		ch,
		chainOutputs,
		timestamp,
		[]isc.Request{req},
		estimateGasMode,
		nil,
	)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, errors.New("request was skipped")
	}
	return results[0], nil
}
