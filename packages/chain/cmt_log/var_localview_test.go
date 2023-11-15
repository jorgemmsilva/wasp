// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package cmt_log_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/chain/cmt_log"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
)

func TestVarLocalView(t *testing.T) {
	log := testlogger.NewLogger(t)
	defer log.Sync()
	j := cmt_log.NewVarLocalView(-1, func(ao *isc.ChainOutputs) {}, log)
	require.Nil(t, j.Value())
	tipAO, ok, _ := j.AnchorOutputConfirmed(&isc.ChainOutputs{
		AnchorOutput:   &iotago.AnchorOutput{},
		AnchorOutputID: iotago.OutputID{},
	})
	require.True(t, ok)
	require.NotNil(t, tipAO)
	require.NotNil(t, j.Value())
	require.Equal(t, tipAO, j.Value())
}
