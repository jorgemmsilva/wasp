package isc

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/kv/dict"
)

type RequestJSON struct {
	Allowance     *AssetsJSON    `json:"allowance" swagger:"required"`
	CallTarget    CallTargetJSON `json:"callTarget" swagger:"required"`
	Assets        *AssetsJSON    `json:"fungibleTokens" swagger:"required"`
	GasBudget     string         `json:"gasBudget,string" swagger:"required,desc(The gas budget (uint64 as string))"`
	IsEVM         bool           `json:"isEVM" swagger:"required"`
	IsOffLedger   bool           `json:"isOffLedger" swagger:"required"`
	NFT           *NFTJSON       `json:"nft" swagger:"required"`
	Params        dict.JSONDict  `json:"params" swagger:"required"`
	RequestID     string         `json:"requestId" swagger:"required"`
	SenderAccount string         `json:"senderAccount" swagger:"required"`
	TargetAddress string         `json:"targetAddress" swagger:"required"`
}

func RequestToJSONObject(request Request, l1API iotago.API) RequestJSON {
	gasBudget, isEVM := request.GasBudget()
	msg := request.Message()
	return RequestJSON{
		Allowance:     assetsToJSONObject(request.Allowance()),
		CallTarget:    callTargetToJSONObject(msg.Target),
		Assets:        assetsToJSONObject(request.Assets()),
		GasBudget:     strconv.FormatUint(gasBudget, 10),
		IsEVM:         isEVM,
		IsOffLedger:   request.IsOffLedger(),
		NFT:           NFTToJSONObject(request.NFT(), l1API),
		Params:        msg.Params.JSONDict(),
		RequestID:     request.ID().String(),
		SenderAccount: request.SenderAccount().Bech32(l1API.ProtocolParameters().Bech32HRP()),
		TargetAddress: request.TargetAddress().Bech32(l1API.ProtocolParameters().Bech32HRP()),
	}
}

func RequestToJSON(req Request, l1API iotago.API) ([]byte, error) {
	return json.Marshal(RequestToJSONObject(req, l1API))
}

// ----------------------------------------------------------------------------

type AssetsJSON struct {
	BaseTokens   string             `json:"baseTokens" swagger:"required,desc(The base tokens (uint64 as string))"`
	NativeTokens NativeTokenMapJSON `json:"nativeTokens" swagger:"required"`
	NFTs         []string           `json:"nfts" swagger:"required"`
}

func assetsToJSONObject(assets *Assets) *AssetsJSON {
	if assets == nil {
		return nil
	}

	ret := &AssetsJSON{
		BaseTokens:   strconv.FormatUint(uint64(assets.BaseTokens), 10),
		NativeTokens: NativeTokenMapToJSONObject(assets.NativeTokens),
		NFTs:         make([]string, len(assets.NFTs)),
	}

	for k, v := range assets.NFTs {
		ret.NFTs[k] = v.ToHex()
	}
	return ret
}

// ----------------------------------------------------------------------------

type NFTJSON struct {
	ID       string         `json:"id" swagger:"required"`
	Issuer   string         `json:"issuer" swagger:"required"`
	Metadata map[string]any `json:"metadata" swagger:"required"`
	Owner    string         `json:"owner" swagger:"required"`
}

func NFTToJSONObject(nft *NFT, l1API iotago.API) *NFTJSON {
	if nft == nil {
		return nil
	}

	ownerString := ""
	if nft.Owner != nil {
		ownerString = nft.Owner.Bech32(l1API.ProtocolParameters().Bech32HRP())
	}

	issuerString := ""
	if nft.Issuer != nil {
		issuerString = nft.Issuer.Bech32(l1API.ProtocolParameters().Bech32HRP())
	}

	return &NFTJSON{
		ID:       nft.ID.ToHex(),
		Issuer:   issuerString,
		Metadata: lo.Must(l1API.Underlying().MapEncode(context.Background(), nft.Metadata)).Values(),
		Owner:    ownerString,
	}
}

// ----------------------------------------------------------------------------

type NativeTokenMapJSON map[string]string

/*
type NativeTokenMapJSON struct {
	ID     string `json:"id" swagger:"required"`
	Amount string `json:"amount" swagger:"required"`
}
*/

func NativeTokenMapToJSONObject(tokens iotago.NativeTokenSum) NativeTokenMapJSON {
	nativeTokens := NativeTokenMapJSON{}

	for id, n := range tokens {
		nativeTokens[id.ToHex()] = n.String()
	}

	return nativeTokens
}

// ----------------------------------------------------------------------------

type CallTargetJSON struct {
	ContractHName string `json:"contractHName" swagger:"desc(The contract name as HName (Hex)),required"`
	FunctionHName string `json:"functionHName" swagger:"desc(The function name as HName (Hex)),required"`
}

func callTargetToJSONObject(target CallTarget) CallTargetJSON {
	return CallTargetJSON{
		ContractHName: target.Contract.String(),
		FunctionHName: target.EntryPoint.String(),
	}
}
