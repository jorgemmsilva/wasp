// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package chain

import (
	"context"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
)

type NodeConnection interface {
	ChainNodeConn
	Run(ctx context.Context) error
	WaitUntilInitiallySynced(context.Context) error
	Bech32HRP() iotago.NetworkPrefix
	L1API() iotago.API
	BaseTokenInfo() *api.InfoResBaseToken
}
