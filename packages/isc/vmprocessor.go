// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package isc

import (
	"errors"
)

// package present processor interface. It must be implemented by VM

// VMProcessor is an interface to the VM processor instance.
type VMProcessor interface {
	GetEntryPoint(code Hname) (VMProcessorEntryPoint, bool)
	GetDescription() string
}

// VMProcessorEntryPoint is an abstract interface by which VM is called by passing
// the Sandbox interface
type VMProcessorEntryPoint interface {
	Call(ctx interface{}) []byte
	IsView() bool
}

var ErrWrongTypeEntryPoint = errors.New("wrong type of the entry point")
