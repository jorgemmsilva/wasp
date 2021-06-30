// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
	"github.com/stretchr/testify/require"
)

func TestBlobPutGet(t *testing.T) {
	log := testlogger.NewLogger(t)
	reg := registry.NewRegistry(log, mapdb.NewMapDB())

	data := []byte("data-data-data-data-data-data-data-data-data")
	h := hashing.HashData(data)

	hback, err := reg.PutBlob(data)
	require.NoError(t, err)
	require.EqualValues(t, h, hback)

	back, ok, err := reg.GetBlob(h)
	require.NoError(t, err)
	require.True(t, ok)
	require.EqualValues(t, data, back)
}
