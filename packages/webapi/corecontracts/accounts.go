package corecontracts

import (
	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"

	"github.com/iotaledger/wasp/packages/vm/core/accounts"
)

func GetAccounts(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) ([]isc.AgentID, error) {
	chainID, res, err := callViewInvoker(accounts.ViewAccounts.Message(), blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}
	return accounts.ViewAccounts.Output.Decode(res, chainID)
}

func GetTotalAssets(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) (*isc.FungibleTokens, error) {
	_, ret, err := callViewInvoker(accounts.ViewTotalAssets.Message(), blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}
	return accounts.ViewTotalAssets.Output.Decode(ret)
}

func GetAccountBalance(callViewInvoker CallViewInvoker, agentID isc.AgentID, blockIndexOrTrieRoot string) (*isc.FungibleTokens, error) {
	_, ret, err := callViewInvoker(accounts.ViewBalance.Message(agentID), blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}
	return accounts.ViewBalance.Output.Decode(ret)
}

func GetAccountNFTs(callViewInvoker CallViewInvoker, agentID isc.AgentID, blockIndexOrTrieRoot string) ([]iotago.NFTID, error) {
	_, res, err := callViewInvoker(accounts.ViewAccountNFTs.Message(agentID), blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}
	return accounts.ViewAccountNFTs.Output.Decode(res), nil
}

func GetAccountFoundries(callViewInvoker CallViewInvoker, agentID isc.AgentID, blockIndexOrTrieRoot string) ([]uint32, error) {
	_, ret, err := callViewInvoker(
		accounts.ViewAccountFoundries.Message(agentID),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return nil, err
	}
	sns, err := accounts.ViewAccountFoundries.Output.Decode(ret)
	if err != nil {
		return nil, err
	}
	return lo.Keys(sns), nil
}

func GetAccountNonce(callViewInvoker CallViewInvoker, agentID isc.AgentID, blockIndexOrTrieRoot string) (uint64, error) {
	_, ret, err := callViewInvoker(
		accounts.ViewGetAccountNonce.Message(agentID),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return 0, err
	}
	return accounts.ViewGetAccountNonce.Output.Decode(ret)
}

func GetNFTData(callViewInvoker CallViewInvoker, nftID iotago.NFTID, blockIndexOrTrieRoot string) (*isc.NFT, error) {
	_, ret, err := callViewInvoker(accounts.ViewNFTData.Message(nftID), blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}
	return accounts.ViewNFTData.Output.Decode(ret)
}

func GetNativeTokenIDRegistry(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) ([]iotago.NativeTokenID, error) {
	_, res, err := callViewInvoker(accounts.ViewGetNativeTokenIDRegistry.Message(), blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}
	return accounts.ViewGetNativeTokenIDRegistry.Output.Decode(res), nil
}

func GetFoundryOutput(callViewInvoker CallViewInvoker, l1Api iotago.API, serialNumber uint32, blockIndexOrTrieRoot string) (*iotago.FoundryOutput, error) {
	_, res, err := callViewInvoker(accounts.ViewFoundryOutput.Message(serialNumber), blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}
	out, err := accounts.ViewFoundryOutput.Output.Decode(res)
	if err != nil {
		return nil, err
	}
	return out.(*iotago.FoundryOutput), nil
}
