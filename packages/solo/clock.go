// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package solo

import (
	iotago "github.com/iotaledger/iota.go/v4"
)

func (env *Solo) SlotIndex() iotago.SlotIndex {
	return env.utxoDB.SlotIndex()
}

func (env *Solo) AdvanceSlotIndex(step uint) {
	env.utxoDB.AdvanceSlotIndex(step)
}
