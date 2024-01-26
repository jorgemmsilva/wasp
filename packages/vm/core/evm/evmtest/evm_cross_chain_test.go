// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package evmtest

import (
	"crypto/ecdsa"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/evm"
	"github.com/iotaledger/wasp/packages/vm/core/evm/iscmagic"
)

func newISCMagicSandboxInstance(t *testing.T, env *solo.Solo, chain *solo.Chain) *IscContractInstance {
	parsedABI, err := abi.JSON(strings.NewReader(iscmagic.SandboxABI))
	require.NoError(t, err)

	envSoloCh := &SoloChainEnv{
		t:          t,
		solo:       env,
		Chain:      chain,
		evmChainID: evm.DefaultChainID,
		evmChain:   chain.EVM(),
	}

	return &IscContractInstance{
		EVMContractInstance: &EVMContractInstance{
			chain:         envSoloCh,
			defaultSender: nil,
			address:       iscmagic.Address,
			abi:           parsedABI,
		},
	}
}

func newERC20ContractInstance(t *testing.T, env *solo.Solo, chain *solo.Chain, ERC20contractAddr common.Address) *IscContractInstance {
	erc20BaseABI, err := abi.JSON(strings.NewReader(iscmagic.ERC20ExternalNativeTokensABI))
	require.NoError(t, err)

	envSoloCh := &SoloChainEnv{
		t:          t,
		solo:       env,
		Chain:      chain,
		evmChainID: evm.DefaultChainID,
		evmChain:   chain.EVM(),
	}

	return &IscContractInstance{
		EVMContractInstance: &EVMContractInstance{
			chain:         envSoloCh,
			defaultSender: nil,
			address:       ERC20contractAddr,
			abi:           erc20BaseABI,
		},
	}
}

func sandboxCall(t *testing.T, wallet *ecdsa.PrivateKey, sandboxContract *IscContractInstance, contract isc.Hname, entrypoint isc.Hname, params dict.Dict, allowance uint64) {
	evmParams := &iscmagic.ISCDict{}
	for k, v := range params {
		evmParams.Items = append(evmParams.Items, iscmagic.ISCDictItem{Key: []byte(k), Value: v})
	}
	_, err := sandboxContract.CallFn(
		[]ethCallOptions{{sender: wallet}},
		"call",
		contract,
		entrypoint,
		evmParams,
		&iscmagic.ISCAssets{
			BaseTokens: allowance,
		},
	)
	require.NoError(t, err)
}

func TestCrossChainShennanigans(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{
		AutoAdjustStorageDeposit: true,
		Debug:                    true,
		PrintStackTrace:          true,
		GasBurnLogEnabled:        false,
	})

	//
	// create 2 chains, we are going to create a foundry/ERC20 on chainA, and transfer some tokens to chainB
	chainA, _ := env.NewChainExt(nil, 0, "chainA")
	chainB, _ := env.NewChainExt(nil, 0, "chainB")

	// some token settings
	const (
		tokenName         = "foo bar token"
		tokenTickerSymbol = "FBT"
		tokenDecimals     = 8
	)

	//
	// deposit some funds to an addr on chainA
	evmWallet, evmAddr := chainA.NewEthereumAccountWithL2Funds()

	//
	// create the foundry on chainA
	supply := big.NewInt(int64(10 * isc.Million))
	sandboxContractChainA := newISCMagicSandboxInstance(t, env, chainA)

	sandboxCall(t, evmWallet, sandboxContractChainA,
		accounts.Contract.Hname(),
		accounts.FuncFoundryCreateNew.Hname(),
		dict.Dict{
			accounts.ParamTokenScheme: codec.EncodeTokenScheme(&iotago.SimpleTokenScheme{
				MaximumSupply: supply,
				MeltedTokens:  big.NewInt(0),
				MintedTokens:  big.NewInt(0),
			}),
		},
		1*isc.Million, // allowance necessary to cover the foundry creation SD
	)

	// TODO here we know that the SN must be 1. An ethereum contract calling "FuncFoundryCreateNew" would have to save the return value of that function call and persis the obtained foundrySN into it's state
	foundrySN := uint32(1)
	nativeTokenID, err := chainA.GetNativeTokenIDByFoundrySN(foundrySN)
	require.NoError(t, err)

	//
	// mint some tokens on chainA (only the foundry owner can mint tokens)
	sandboxCall(t, evmWallet, sandboxContractChainA,
		accounts.Contract.Hname(),
		accounts.FuncFoundryModifySupply.Hname(),
		dict.Dict{
			accounts.ParamFoundrySN:      codec.Encode(foundrySN),
			accounts.ParamSupplyDeltaAbs: codec.Encode(supply), // mint the entire supply
		},
		1*isc.Million, // allowance necessary to cover the accounting UTXO created for a first time a new kind of NT is minted
	)

	//
	// register the foundry on chainA (I really want to do this automatically upon foundry creation in the future)
	sandboxCall(t, evmWallet, sandboxContractChainA,
		evm.Contract.Hname(),
		evm.FuncRegisterERC20NativeToken.Hname(),
		dict.Dict{
			evm.FieldFoundrySN:         codec.EncodeUint32(foundrySN),
			evm.FieldTokenName:         codec.EncodeString(tokenName),
			evm.FieldTokenTickerSymbol: codec.EncodeString(tokenTickerSymbol),
			evm.FieldTokenDecimals:     codec.EncodeUint8(tokenDecimals),
		},
		0, // no allowance necessary
	)

	//
	// ERC20 contract should now be available on chainA, and the supply should be owner by the minter
	erc20ContractAddressChainA := iscmagic.ERC20NativeTokensAddress(foundrySN)
	ERC20ContractChainA := newERC20ContractInstance(t, env, chainA, erc20ContractAddressChainA)
	var balance *big.Int
	require.NoError(t, ERC20ContractChainA.callView("balanceOf", []interface{}{evmAddr}, &balance))
	require.EqualValues(t, supply, balance)

	//
	// register the foundry on chain B (request is sent from chainA)

	// save chainB current block, so we can know when it processes the register req
	chainBBlockIndex := chainB.GetLatestBlockInfo().BlockIndex()

	// chainA itself will create a request targeting chainB
	// this request must be done by the foundry owner (the foundry creator in this case)
	sandboxCall(t, evmWallet, sandboxContractChainA,
		evm.Contract.Hname(),
		evm.FuncRegisterERC20NativeTokenOnRemoteChain.Hname(),
		dict.Dict{
			evm.FieldFoundrySN:         codec.EncodeUint32(foundrySN),
			evm.FieldTokenName:         codec.EncodeString(tokenName),
			evm.FieldTokenTickerSymbol: codec.EncodeString(tokenTickerSymbol),
			evm.FieldTokenDecimals:     codec.EncodeUint8(tokenDecimals),
			evm.FieldTargetAddress:     codec.EncodeAddress(chainB.ChainID.AsAddress()), // the target chain is chain B
		},
		1*isc.Million, // provide funds for cross-chain request SD
	)

	// wait until chainB handles the request, assert it was processed successfully
	chainB.WaitUntil(func() bool {
		return chainB.GetLatestBlockInfo().BlockIndex() > chainBBlockIndex
	})
	lastBlockReceipts := chainB.GetRequestReceiptsForBlock()
	require.Len(t, lastBlockReceipts, 1)
	require.Nil(t, lastBlockReceipts[0].Error)

	//
	// send some tokens to an arbitrary EVM account on chainB
	_, targetEVMAddr := chainB.NewEthereumAccountWithL2Funds()
	nativeTokenAmountToSend := big.NewInt(50)

	// metadata to be executed on the target chain
	metadata := iscmagic.WrapISCSendMetadata(
		isc.SendMetadata{
			TargetContract: accounts.Contract.Hname(),
			EntryPoint:     accounts.FuncTransferAllowanceTo.Hname(),
			Params: dict.Dict{
				accounts.ParamAgentID: codec.Encode(isc.NewEthereumAddressAgentID(chainB.ChainID, targetEVMAddr)),
			},
			Allowance: &isc.Assets{
				NativeTokens: []*iotago.NativeToken{
					{ID: nativeTokenID, Amount: nativeTokenAmountToSend}, // specify the token to be transferred here
				},
			},
			GasBudget: math.MaxUint64, // allow all gas that can be used
		},
	)

	// save chainB current block, so we can know when it processes the transfer req
	chainBBlockIndex = chainB.GetLatestBlockInfo().BlockIndex()

	_, err = sandboxContractChainA.CallFn(
		[]ethCallOptions{{sender: evmWallet}},
		"send",
		iscmagic.WrapL1Address(chainB.ChainID.AsAddress()), // target of the "send" call is chainB
		iscmagic.WrapISCAssets(
			&isc.Assets{
				BaseTokens: 1 * isc.Million, // must add some base tokens in order to pay for the gas on the target chain
				NativeTokens: []*iotago.NativeToken{
					{ID: nativeTokenID, Amount: nativeTokenAmountToSend}, // specify the token to be transferred here
				},
			},
		),
		false,
		metadata,
		iscmagic.ISCSendOptions{},
	)
	require.NoError(t, err)

	// wait until chainB handles the request, assert it was processed successfully
	chainB.WaitUntil(func() bool {
		return chainB.GetLatestBlockInfo().BlockIndex() > chainBBlockIndex
	})
	lastBlockReceipts = chainB.GetRequestReceiptsForBlock()
	require.Len(t, lastBlockReceipts, 1)
	require.Nil(t, lastBlockReceipts[0].Error)

	// isc accounting has the native token owned by the correct agentID
	iscAssets := chainB.L2Assets(isc.NewEthereumAddressAgentID(chainB.ChainID, targetEVMAddr))
	require.Len(t, iscAssets.NativeTokens, 1)
	require.EqualValues(t, iscAssets.NativeTokens[0].Amount, nativeTokenAmountToSend)

	// check that the ERC20 contract on ChainB acknowledges that the target address has some tokens
	// get the EVM address of the ERC20 contract:
	var erc20ContractAddressChainB common.Address
	res, err := chainB.CallView(evm.Contract.Name, evm.FuncGetERC20ExternalNativeTokenAddress.Name,
		evm.FieldNativeTokenID, nativeTokenID[:],
	)
	require.NoError(t, err)
	require.NotZero(t, len(res[evm.FieldResult]))
	copy(erc20ContractAddressChainB[:], res[evm.FieldResult])

	ERC20ContractChainB := newERC20ContractInstance(t, env, chainB, erc20ContractAddressChainB)

	var name string
	require.NoError(t, ERC20ContractChainB.callView("name", nil, &name))
	require.Equal(t, tokenName, name)

	var balanceOnChainB *big.Int
	require.NoError(t, ERC20ContractChainB.callView("balanceOf", []interface{}{targetEVMAddr}, &balanceOnChainB))
	require.EqualValues(t, nativeTokenAmountToSend, balanceOnChainB)
}
