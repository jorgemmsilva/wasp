package chainutil

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/wasp/packages/chain/chaintypes"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/vm/core/evm"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

// EVMCall executes an EVM contract call and returns its output, discarding any state changes
func EVMCall(
	ch chaintypes.ChainCore,
	chainOutputs *isc.ChainOutputs,
	call ethereum.CallMsg,
	l1API iotago.API,
	tokenInfo api.InfoResBaseToken,
) ([]byte, error) {
	info := getChainInfo(ch)

	// 0 means view call
	gasLimit := gas.EVMCallGasLimit(info.GasLimits, &info.GasFeePolicy.EVMGasRatio)
	if call.Gas != 0 && call.Gas > gasLimit {
		call.Gas = gasLimit
	}

	if call.GasPrice == nil {
		call.GasPrice = info.GasFeePolicy.GasPriceWei(tokenInfo.Decimals)
	}

	iscReq := isc.NewEVMOffLedgerCallRequest(ch.ID(), call)
	// TODO: setting EstimateGasMode = true feels wrong here
	timestamp := time.Now()
	res, err := runISCRequest(ch, chainOutputs, timestamp, iscReq, true, l1API, tokenInfo)
	if err != nil {
		return nil, err
	}
	if res.Receipt.Error != nil {
		vmerr, resolvingErr := ResolveError(ch, res.Receipt.Error)
		if resolvingErr != nil {
			panic(fmt.Errorf("error resolving vmerror %w", resolvingErr))
		}
		return nil, vmerr
	}
	return res.Return[evm.FieldResult], nil
}
