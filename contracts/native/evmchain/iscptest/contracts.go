// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package iscptest

import (
	_ "embed"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

var (
	//go:embed ISCPTest.abi
	ISCPTestContractABI string
	//go:embed ISCPTest.bin
	iscpTestContractBytecodeHex string
	ISCPTestContractBytecode    = common.FromHex(strings.TrimSpace(iscpTestContractBytecodeHex))
)
