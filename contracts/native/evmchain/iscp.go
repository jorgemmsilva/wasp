// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package evmchain

import (
	_ "embed"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/wasp/packages/iscp"
)

var (
	ISCPContractAddress = common.HexToAddress("0x1074")
	//go:embed iscp/iscp.abi.json
	ISCPContractABI string
	//go:embed iscp/iscp.bytecode.hex
	iscpContractBytecodeHex string
)

func ChainIDToEVMHash(chainID *iscp.ChainID) (ret common.Hash) {
	copy(ret[:], chainID.AliasAddress.Digest())
	return ret
}

func ChainIDFromEVMHash(hash common.Hash) *iscp.ChainID {
	var addressBytes []byte
	addressBytes = append(addressBytes, byte(ledgerstate.AliasAddressType))
	addressBytes = append(addressBytes, hash[:]...)
	chainID, err := iscp.ChainIDFromBytes(addressBytes)
	if err != nil {
		panic("could not create ChainID")
	}
	return chainID
}

func iscpGenesisAccount(chainID *iscp.ChainID) core.GenesisAccount {
	return core.GenesisAccount{
		Code: common.FromHex(strings.TrimSpace(iscpContractBytecodeHex)),
		Storage: map[common.Hash]common.Hash{
			// state variable at slot 0 is chainID
			{}: ChainIDToEVMHash(chainID),
		},
		Balance: &big.Int{},
	}
}
