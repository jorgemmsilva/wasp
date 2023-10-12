// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package cmt_log

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
	return fmt.Sprintf("{cmtLog.inputAccountOutputConfirmed, %v}", inp.accountOutput)
}
