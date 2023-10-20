package parameters

import (
	"os"
	"strings"
	"sync"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/nodeclient/apimodels"
	"github.com/iotaledger/iota.go/v4/tpkg"
)

// L1Params describes parameters coming from the L1Params node
type L1Params struct {
	Protocol  iotago.ProtocolParameters   `json:"protocol" swagger:"required"`
	BaseToken *apimodels.InfoResBaseToken `json:"baseToken" swagger:"required"`
}

func isTestContext() bool {
	return strings.HasSuffix(os.Args[0], ".test") ||
		strings.HasSuffix(os.Args[0], ".test.exe") ||
		strings.Contains(os.Args[0], "__debug_bin")
}

var L1ForTesting = &L1Params{
	Protocol: tpkg.TestAPI.ProtocolParameters(),
	BaseToken: &apimodels.InfoResBaseToken{
		Name:            "TestCoin",
		TickerSymbol:    "TEST",
		Unit:            "TEST",
		Subunit:         "testies",
		Decimals:        6,
		UseMetricPrefix: false,
	},
}

var L1 = sync.OnceValue(func() *L1Params {
	if !isTestContext() {
		panic("call InitL1() first")
	}
	return L1ForTesting
})

// InitL1Lazy sets a function to be called the first time L1() is called.
func InitL1Lazy(f func() *L1Params) {
	L1 = sync.OnceValue(f)
}

func InitL1(l1 *L1Params) {
	L1 = func() *L1Params {
		return l1
	}
}

var L1API = sync.OnceValue(func() iotago.API {
	return iotago.V3API(L1().Protocol)
})

func RentStructure() *iotago.RentStructure {
	return L1API().RentStructure()
}

func Protocol() iotago.ProtocolParameters {
	return L1().Protocol
}

func NetworkPrefix() iotago.NetworkPrefix {
	return Protocol().Bech32HRP()
}

func BaseToken() *apimodels.InfoResBaseToken {
	return L1().BaseToken
}
