// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package iscpcontract

import (
	"crypto/ecdsa"
	_ "embed"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/wasp/packages/evm"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/iscp/assert"
)

var (
	// ISCPContractAddress is the arbitrary address on which the standard
	// ISCP EVM contract lives
	EVMAddress = common.HexToAddress("0x1074")
	//go:embed ISCP.abi
	ABIJSON string
	ABI     = parseABI(ABIJSON)

	//go:embed ISCP.bin-runtime
	bytecodeHex string

	// the sender account for updating the ISCP contract state
	sender = initSender()
)

func initSender() *ecdsa.PrivateKey {
	sender, err := crypto.HexToECDSA("0000000000000000000000000000000000000000000000000000000000001074")
	if err != nil {
		panic(err)
	}
	return sender
}

func parseABI(abiJSON string) abi.ABI {
	abi, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		panic(err)
	}
	return abi
}

// ISCPAddress maps to the equally-named struct in iscp.sol
type ISCPAddress struct {
	TypeID [1]byte
	Digest [32]byte
}

func ChainIDToISCPAddress(chainID *iscp.ChainID) (ret ISCPAddress) {
	ret.TypeID[0] = byte(chainID.AliasAddress.Type())
	copy(ret.Digest[:], chainID.AliasAddress.Digest())
	return ret
}

func ChainIDFromISCPAddress(a ISCPAddress) *iscp.ChainID {
	if a.TypeID[0] != byte(ledgerstate.AliasAddressType) {
		panic(fmt.Sprintf("expected type id %d, got %d", ledgerstate.AliasAddressType, a.TypeID[0]))
	}
	var addressBytes []byte
	addressBytes = append(addressBytes, a.TypeID[0])
	addressBytes = append(addressBytes, a.Digest[:]...)
	chainID, err := iscp.ChainIDFromBytes(addressBytes)
	if err != nil {
		// should not happen
		panic(err.Error())
	}
	return chainID
}

// InitGenesisAccount sets up the initial state of the ISCP EVM contract
// which will go into the EVM genesis block
func InitGenesisAccount(genesisAlloc core.GenesisAlloc, chainID *iscp.ChainID) {
	chainIDAsISCPAddress := ChainIDToISCPAddress(chainID)
	var typeIDHash common.Hash
	typeIDHash[31] = chainIDAsISCPAddress.TypeID[0]
	var digestHash common.Hash
	copy(digestHash[:], chainIDAsISCPAddress.Digest[:])

	genesisAlloc[EVMAddress] = core.GenesisAccount{
		Code: common.FromHex(strings.TrimSpace(bytecodeHex)),
		Storage: map[common.Hash]common.Hash{
			// offset 0 / slot 0: chainID.typeId
			common.HexToHash("00"): typeIDHash,
			// offset 0 / slot 1: chainID.digest
			common.HexToHash("01"): digestHash,
		},
		Balance: &big.Int{},
	}
}

// UpdateState is called right after starting a new EVM block, and it updates
// the ISCP contract state
func UpdateState(ctx iscp.Sandbox, emu *evm.EVMEmulator) {
	a := assert.NewAssert(ctx.Log())

	senderAddress := crypto.PubkeyToAddress(sender.PublicKey)

	amount := big.NewInt(0)
	nonce, err := emu.NonceAt(senderAddress, nil)
	a.RequireNoError(err)

	var entropy common.Hash
	copy(entropy[:], ctx.GetEntropy().Bytes())
	callArguments, err := ABI.Pack("setEntropy", entropy)
	a.RequireNoError(err)

	gas := evm.GasLimitDefault

	tx, err := types.SignTx(
		types.NewTransaction(nonce, EVMAddress, amount, gas, evm.GasPrice, callArguments),
		emu.Signer(),
		sender,
	)
	a.RequireNoError(err)

	receipt, err := emu.SendTransaction(tx)

	a.RequireNoError(err)
	a.Require(receipt.Status == types.ReceiptStatusSuccessful, "iscp state update tx failed")
}
