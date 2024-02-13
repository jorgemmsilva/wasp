package providers

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp-wallet-sdk/types"
)

func MapCoinType(prefix iotago.NetworkPrefix) types.CoinType {
	switch prefix {
	case iotago.PrefixMainnet:
		return types.CoinTypeIOTA
	case iotago.PrefixShimmer, iotago.PrefixTestnet:
		return types.CoinTypeSMR
	default:
		// For now returns SMR as default, but keep the logic above in case things change.
		return types.CoinTypeSMR
	}
}
