package corecontracts

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/kv/dict"

	"github.com/iotaledger/wasp/packages/vm/core/accounts"
)

func GetAccounts(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) ([]isc.AgentID, error) {
	chainID, accountIDs, err := callViewInvoker(accounts.Contract.Hname(), accounts.ViewAccounts.Hname(), nil, blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}

	ret := make([]isc.AgentID, 0)
	for accountID := range accountIDs {
		agentID, err := accounts.AgentIDFromKey(accountID, chainID)
		if err != nil {
			return nil, err
		}
		ret = append(ret, agentID)
	}
	return ret, nil
}

func GetTotalAssets(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) (*isc.FungibleTokens, error) {
	_, ret, err := callViewInvoker(accounts.Contract.Hname(), accounts.ViewTotalAssets.Hname(), nil, blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}

	return isc.FungibleTokensFromDict(ret)
}

func GetAccountBalance(callViewInvoker CallViewInvoker, agentID isc.AgentID, blockIndexOrTrieRoot string) (*isc.FungibleTokens, error) {
	_, ret, err := callViewInvoker(accounts.Contract.Hname(),
		accounts.ViewBalance.Hname(), codec.MakeDict(map[string]interface{}{
			accounts.ParamAgentID: agentID,
		}),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return nil, err
	}

	return isc.FungibleTokensFromDict(ret)
}

func GetAccountNFTs(callViewInvoker CallViewInvoker, agentID isc.AgentID, blockIndexOrTrieRoot string) ([]iotago.NFTID, error) {
	_, res, err := callViewInvoker(
		accounts.Contract.Hname(),
		accounts.ViewAccountNFTs.Hname(),
		codec.MakeDict(map[string]interface{}{
			accounts.ParamAgentID: agentID,
		},
		),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return nil, err
	}

	nftIDs := collections.NewArrayReadOnly(res, accounts.ParamNFTIDs)
	ret := make([]iotago.NFTID, nftIDs.Len())
	for i := range ret {
		copy(ret[i][:], nftIDs.GetAt(uint32(i)))
	}
	return ret, nil
}

func GetAccountFoundries(callViewInvoker CallViewInvoker, agentID isc.AgentID, blockIndexOrTrieRoot string) ([]uint32, error) {
	_, foundrySNs, err := callViewInvoker(
		accounts.Contract.Hname(),
		accounts.ViewAccountFoundries.Hname(),
		dict.Dict{
			accounts.ParamAgentID: codec.AgentID.Encode(agentID),
		},
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return nil, err
	}
	ret := make([]uint32, 0, len(foundrySNs))
	for foundrySN := range foundrySNs {
		sn, err := codec.Uint32.Decode([]byte(foundrySN))
		if err != nil {
			return nil, err
		}
		ret = append(ret, sn)
	}
	return ret, nil
}

func GetAccountNonce(callViewInvoker CallViewInvoker, agentID isc.AgentID, blockIndexOrTrieRoot string) (uint64, error) {
	_, ret, err := callViewInvoker(
		accounts.Contract.Hname(),
		accounts.ViewGetAccountNonce.Hname(),
		codec.MakeDict(map[string]interface{}{
			accounts.ParamAgentID: agentID,
		}),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return 0, err
	}

	nonce := ret.Get(accounts.ParamAccountNonce)

	return codec.Uint64.Decode(nonce)
}

func GetNFTData(callViewInvoker CallViewInvoker, nftID iotago.NFTID, blockIndexOrTrieRoot string) (*isc.NFT, error) {
	_, ret, err := callViewInvoker(
		accounts.Contract.Hname(),
		accounts.ViewNFTData.Hname(),
		codec.MakeDict(map[string]interface{}{
			accounts.ParamNFTID: nftID[:],
		}),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return nil, err
	}

	nftData, err := isc.NFTFromBytes(ret.Get(accounts.ParamNFTData))
	if err != nil {
		return nil, err
	}

	return nftData, nil
}

func GetNativeTokenIDRegistry(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) ([]iotago.NativeTokenID, error) {
	_, nativeTokenIDs, err := callViewInvoker(accounts.Contract.Hname(), accounts.ViewGetNativeTokenIDRegistry.Hname(), nil, blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}

	ret := make([]iotago.NativeTokenID, 0, len(nativeTokenIDs))
	for nativeTokenID := range nativeTokenIDs {
		tokenID, err := isc.NativeTokenIDFromBytes([]byte(nativeTokenID))
		if err != nil {
			return nil, err
		}
		ret = append(ret, tokenID)
	}

	return ret, nil
}

func GetFoundryOutput(callViewInvoker CallViewInvoker, l1Api iotago.API, serialNumber uint32, blockIndexOrTrieRoot string) (*iotago.FoundryOutput, error) {
	_, res, err := callViewInvoker(accounts.Contract.Hname(),
		accounts.ViewFoundryOutput.Hname(), codec.MakeDict(map[string]interface{}{accounts.ParamFoundrySN: serialNumber}),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return nil, err
	}

	outBin := res.Get(accounts.ParamFoundryOutputBin)
	out := &iotago.FoundryOutput{}

	// TODO: <lmoe> Did this really change to L1API.Decode?
	_, err = l1Api.Decode(outBin, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}
