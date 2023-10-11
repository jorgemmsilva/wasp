package parameters

import (
	"os"
	"strings"
	"sync"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/tpkg"
)

// L1Params describes parameters coming from the L1Params node
type L1Params struct {
	Protocol  iotago.ProtocolParameters `json:"protocol" swagger:"required"`
	BaseToken *BaseToken                `json:"baseToken" swagger:"required"`
}

type BaseToken struct {
	Name            string `json:"name" swagger:"desc(The base token name),required"`
	TickerSymbol    string `json:"tickerSymbol" swagger:"desc(The ticker symbol),required"`
	Unit            string `json:"unit" swagger:"desc(The token unit),required"`
	Subunit         string `json:"subunit" swagger:"desc(The token subunit),required"`
	Decimals        uint32 `json:"decimals" swagger:"desc(The token decimals),required"`
	UseMetricPrefix bool   `json:"useMetricPrefix" swagger:"desc(Whether or not the token uses a metric prefix),required"`
}

var (
	l1ParamsMutex = &sync.RWMutex{}
	l1Params      *L1Params

	L1ForTesting = &L1Params{
		Protocol: tpkg.TestAPI.ProtocolParameters(),
		BaseToken: &BaseToken{
			Name:            "Iota",
			TickerSymbol:    "MIOTA",
			Unit:            "MIOTA",
			Subunit:         "IOTA",
			Decimals:        6,
			UseMetricPrefix: false,
		},
	}

	l1ParamsLazyInit func()
)

func isTestContext() bool {
	return strings.HasSuffix(os.Args[0], ".test") ||
		strings.HasSuffix(os.Args[0], ".test.exe") ||
		strings.Contains(os.Args[0], "__debug_bin")
}

func L1() *L1Params {
	l1ParamsMutex.Lock()
	defer l1ParamsMutex.Unlock()
	return L1NoLock()
}

func L1NoLock() *L1Params {
	if l1Params == nil {
		if isTestContext() {
			l1Params = L1ForTesting
		} else if l1ParamsLazyInit != nil {
			l1ParamsLazyInit()
		}
	}
	if l1Params == nil {
		panic("call InitL1() first")
	}
	return l1Params
}

// InitL1Lazy sets a function to be called the first time L1() is called.
// The function must call InitL1().
func InitL1Lazy(f func()) {
	l1ParamsLazyInit = f
}

func InitL1(l1 *L1Params) {
	l1Params = l1
}

func L1API() iotago.API {
	return iotago.V3API(L1().Protocol)
}
