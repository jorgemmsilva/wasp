// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package coretypes

import "github.com/iotaledger/goshimmer/packages/ledgerstate"

// ColoredBalances is an interface to immutable map of (color code: int64)
//
// New colored balance can be created by cbalances.NewFromMap and cbalances.NewFromBalances
type ColoredBalances interface {
	// Balance is balance of the color or 0 if color is not present
	Balance(color ledgerstate.Color) int64
	// Iterate over elements
	Iterate(func(color ledgerstate.Color, balance int64) bool)
	// IterateDeterministic iterates over elements in the order of lexicographically sorted keys
	IterateDeterministic(func(color ledgerstate.Color, balance int64) bool)
	// Len number of (non-zero) balances
	Len() uint16
	// Equal returns if balances equal color by color
	Equal(b1 ColoredBalances) bool
	// Diff return difference between receiver and parameter color by color
	Diff(b1 ColoredBalances) ColoredBalances
	// NonNegative is when all elements >= 0
	NonNegative() bool
	// AddToMap adds balances to the map color by color
	AddToMap(m map[ledgerstate.Color]int64)
	// TakeOutColor takes out balance with specified color
	TakeOutColor(col ledgerstate.Color) ColoredBalances
	// String human readable representation of the map
	String() string
}
