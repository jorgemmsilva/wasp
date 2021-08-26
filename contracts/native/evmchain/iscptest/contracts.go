// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package iscptest

import (
	_ "embed"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

var (
	//go:embed iscp-test.abi.json
	ISCPTestContractABI string
	//go:embed iscp-test.bytecode.hex
	iscpTestContractBytecodeHex string
	ISCPTestContractBytecode    = common.FromHex(strings.TrimSpace(iscpTestContractBytecodeHex))
)
