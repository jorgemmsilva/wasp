// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package chainmanager

import (
	"fmt"

	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
)

type inputAccountOutputConfirmed struct {
	accountOutput *isc.AccountOutputWithID
}

func NewInputAccountOutputConfirmed(accountOutput *isc.AccountOutputWithID) gpa.Input {
	return &inputAccountOutputConfirmed{
		accountOutput: accountOutput,
	}
}

func (inp *inputAccountOutputConfirmed) String() string {
	return fmt.Sprintf("{chainMgr.inputAccountOutputConfirmed, %v}", inp.accountOutput)
}
