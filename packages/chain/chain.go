// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package chain

import (
	"context"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/parameters"
)

type NodeConnection interface {
	ChainNodeConn
	Run(ctx context.Context) error
	WaitUntilInitiallySynced(context.Context) error
	GetBech32HRP() iotago.NetworkPrefix
	GetL1Params() *parameters.L1Params
	GetL1ProtocolParams() iotago.ProtocolParameters
}
