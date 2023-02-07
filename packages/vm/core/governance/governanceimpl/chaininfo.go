// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package governanceimpl

import (
	"fmt"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
)

// setChainInfo sets the configuration parameters of the chain
// Input (all optional):
// - ParamMaxBlobSizeUint32         - uint32 maximum size of a blob to be saved in the blob contract.
// - ParamMaxEventSizeUint16        - uint16 maximum size of a single event.
// - ParamMaxEventsPerRequestUint16 - uint16 maximum number of events per request.
// Does not set gas fee policy!
func setChainInfo(ctx isc.Sandbox) []byte {
	ctx.RequireCallerIsChainOwner()

	// max blob size
	maxBlobSize := ctx.Params().MustGetUint32(governance.ParamMaxBlobSizeUint32, 0)
	if maxBlobSize > 0 {
		ctx.State().Set(governance.VarMaxBlobSize, codec.Encode(maxBlobSize))
		ctx.Event(fmt.Sprintf("[updated chain config] max blob size: %d", maxBlobSize))
	}

	// max event size
	maxEventSize := ctx.Params().MustGetUint16(governance.ParamMaxEventSizeUint16, 0)
	if maxEventSize > 0 {
		if maxEventSize < governance.MinEventSize {
			// don't allow to set less than MinEventSize to prevent chain owner from bricking the chain
			maxEventSize = governance.MinEventSize
		}
		ctx.State().Set(governance.VarMaxEventSize, codec.Encode(maxEventSize))
		ctx.Event(fmt.Sprintf("[updated chain config] max event size: %d", maxEventSize))
	}

	// max events per request
	maxEventsPerReq := ctx.Params().MustGetUint16(governance.ParamMaxEventsPerRequestUint16, 0)
	if maxEventsPerReq > 0 {
		if maxEventsPerReq < governance.MinEventsPerRequest {
			maxEventsPerReq = governance.MinEventsPerRequest
		}
		ctx.State().Set(governance.VarMaxEventsPerReq, codec.Encode(maxEventsPerReq))
		ctx.Event(fmt.Sprintf("[updated chain config] max eventsPerRequest: %d", maxEventsPerReq))
	}
	return nil
}

// getChainInfo view returns general info about the chain: chain ID, chain owner ID, limits and default fees
func getChainInfo(ctx isc.SandboxView) []byte {
	return governance.MustGetChainInfo(ctx.StateR()).Bytes()
}

func getMaxBlobSize(ctx isc.SandboxView) []byte {
	maxBlobSize, err := ctx.StateR().Get(governance.VarMaxBlobSize)
	if err != nil {
		ctx.Log().Panicf("error getting max blob size, %v", err)
	}
	return util.MustSerialize(maxBlobSize)
}
