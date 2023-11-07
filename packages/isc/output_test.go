// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package isc_test

import (
	"crypto/rand"
	mathrand "math/rand"
	"testing"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

func TestAnchorOutputWithIDSerialization(t *testing.T) {
	anchorOutput := iotago.AnchorOutput{
		Amount:     iotago.BaseToken(mathrand.Uint64()),
		StateIndex: mathrand.Uint32(),
		// serix deserializes with empty slices instead of nil
		StateMetadata:     []byte{},
		Conditions:        make(iotago.AnchorOutputUnlockConditions, 0),
		Features:          make(iotago.AnchorOutputFeatures, 0),
		ImmutableFeatures: make(iotago.AnchorOutputImmFeatures, 0),
	}
	rand.Read(anchorOutput.AnchorID[:])
	anchorOutputID := iotago.OutputID{}
	rand.Read(anchorOutputID[:])

	accountOutput := iotago.AccountOutput{
		Amount:            iotago.BaseToken(mathrand.Uint64()),
		Conditions:        make(iotago.AccountOutputUnlockConditions, 0),
		Features:          make(iotago.AccountOutputFeatures, 0),
		ImmutableFeatures: make(iotago.AccountOutputImmFeatures, 0),
	}
	rand.Read(accountOutput.AccountID[:])
	accountOutputID := iotago.OutputID{}
	rand.Read(accountOutputID[:])

	o := isc.NewChainOuptuts(
		&anchorOutput,
		anchorOutputID,
		&accountOutput,
		accountOutputID,
	)
	rwutil.ReadWriteTest(t, o, new(isc.ChainOutputs))
	rwutil.BytesTest(t, o, isc.ChainOutputsFromBytes)
}
