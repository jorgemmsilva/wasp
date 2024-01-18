package apiextensions

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/hexutil"
	"github.com/iotaledger/wasp/clients/apiclient"
	"github.com/iotaledger/wasp/packages/isc"
)

func FungibleTokensFromAPIResponse(assetsResponse *apiclient.FungibleTokensResponse) (*isc.FungibleTokens, error) {
	assets := isc.NewEmptyFungibleTokens()

	baseTokens, err := hexutil.DecodeUint64(assetsResponse.BaseTokens)
	if err != nil {
		return nil, err
	}

	assets.BaseTokens = iotago.BaseToken(baseTokens)

	for idHex, amountHex := range assetsResponse.NativeTokens {
		nativeTokenIDHex, err2 := hexutil.DecodeHex(idHex)
		if err2 != nil {
			return nil, err2
		}

		nativeTokenID, err2 := isc.NativeTokenIDFromBytes(nativeTokenIDHex)
		if err2 != nil {
			return nil, err2
		}

		amount, err2 := hexutil.DecodeUint256(amountHex)
		if err2 != nil {
			return nil, err2
		}

		assets.NativeTokens[nativeTokenID] = amount
	}

	return assets, err
}
