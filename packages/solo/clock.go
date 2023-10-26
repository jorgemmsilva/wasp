// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package solo

import (
	"time"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
)

func (env *Solo) BlockTime() isc.BlockTime {
	return isc.BlockTime{
		SlotIndex: env.SlotIndex(),
		Timestamp: env.Timestamp(),
	}
}

func (env *Solo) Timestamp() time.Time {
	return env.utxoDB.Timestamp()
}

func (env *Solo) SlotIndex() iotago.SlotIndex {
	return env.utxoDB.SlotIndex()
}

func (env *Solo) AdvanceTime(slotStep uint, timeStep time.Duration) {
	env.utxoDB.AdvanceTime(slotStep, timeStep)
}
