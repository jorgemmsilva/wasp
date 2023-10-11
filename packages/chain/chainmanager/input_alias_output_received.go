// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package chainmanager

import (
	"fmt"

	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
)

type inputAliasOutputConfirmed struct {
	accountOutput *isc.AccountOutputWithID
}

func NewInputAliasOutputConfirmed(accountOutput *isc.AccountOutputWithID) gpa.Input {
	return &inputAliasOutputConfirmed{
		accountOutput: accountOutput,
	}
}

func (inp *inputAliasOutputConfirmed) String() string {
	return fmt.Sprintf("{chainMgr.inputAliasOutputConfirmed, %v}", inp.accountOutput)
}
