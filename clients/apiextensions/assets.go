package apiextensions

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/hexutil"
	"github.com/iotaledger/wasp/clients/apiclient"
	"github.com/iotaledger/wasp/packages/isc"
)

func AssetsFromAPIResponse(assetsResponse *apiclient.AssetsResponse) (*isc.Assets, error) {
	assets := isc.NewEmptyAssets()

	baseTokens, err := hexutil.DecodeUint64(assetsResponse.BaseTokens)
	if err != nil {
		return nil, err
	}

	assets.BaseTokens = iotago.BaseToken(baseTokens)

	for _, nativeToken := range assetsResponse.NativeTokens {
		nativeTokenIDHex, err2 := hexutil.DecodeHex(nativeToken.Id)
		if err2 != nil {
			return nil, err2
		}

		nativeTokenID, err2 := isc.NativeTokenIDFromBytes(nativeTokenIDHex)
		if err2 != nil {
			return nil, err2
		}

		amount, err2 := hexutil.DecodeUint256(nativeToken.Amount)
		if err2 != nil {
			return nil, err2
		}

		assets.NativeTokens = append(assets.NativeTokens, &isc.NativeTokenAmount{
			ID:     nativeTokenID,
			Amount: amount,
		})
	}

	return assets, err
}
