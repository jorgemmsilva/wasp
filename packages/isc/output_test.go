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

func TestAccountOutputWithIDSerialization(t *testing.T) {
	output := iotago.AccountOutput{
		Amount:     iotago.BaseToken(mathrand.Uint64()),
		StateIndex: mathrand.Uint32(),
		// serix deserializes with empty slices instead of nil
		StateMetadata:     make([]byte, 0),
		Conditions:        make(iotago.AccountOutputUnlockConditions, 0),
		Features:          make(iotago.AccountOutputFeatures, 0),
		ImmutableFeatures: make(iotago.AccountOutputImmFeatures, 0),
	}
	rand.Read(output.AccountID[:])
	outputID := iotago.OutputID{}
	rand.Read(outputID[:])
	aliasOutputWithID := isc.NewAccountOutputWithID(&output, outputID)
	rwutil.ReadWriteTest(t, aliasOutputWithID, new(isc.AccountOutputWithID))
	rwutil.BytesTest(t, aliasOutputWithID, isc.AccountOutputWithIDFromBytes)
}
