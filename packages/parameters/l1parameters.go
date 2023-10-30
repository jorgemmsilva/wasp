package parameters

import (
	"os"
	"strings"
	"sync"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/nodeclient/apimodels"
	"github.com/iotaledger/iota.go/v4/tpkg"
)

const (
	betaPerYear                  float64 = 1 / 3.0
	slotsPerEpochExponent                = 13
	slotDurationSeconds                  = 10
	bitsCount                            = 63
	generationRate                       = 1
	generationRateExponent               = 27
	decayFactorsExponent                 = 32
	decayFactorEpochsSumExponent         = 20
)

type BaseTokenInfo apimodels.InfoResBaseToken

var (
	TestBaseToken = &BaseTokenInfo{
		Name:            "TestCoin",
		TickerSymbol:    "TEST",
		Unit:            "TEST",
		Subunit:         "testies",
		Decimals:        6,
		UseMetricPrefix: false,
	}

	schedulerRate   iotago.WorkScore = 100000
	testProtoParams                  = iotago.NewV3ProtocolParameters(
		iotago.WithNetworkOptions("test", "test"),
		iotago.WithSupplyOptions(tpkg.TestTokenSupply, 100, 1, 10, 100, 100, 100),
		iotago.WithWorkScoreOptions(1, 100, 20, 20, 20, 20, 100, 100, 100, 200),
		iotago.WithTimeProviderOptions(100, slotDurationSeconds, slotsPerEpochExponent),
		iotago.WithManaOptions(bitsCount,
			generationRate,
			generationRateExponent,
			tpkg.ManaDecayFactors(betaPerYear, 1<<slotsPerEpochExponent, slotDurationSeconds, decayFactorsExponent),
			decayFactorsExponent,
			tpkg.ManaDecayFactorEpochsSum(betaPerYear, 1<<slotsPerEpochExponent, slotDurationSeconds, decayFactorEpochsSumExponent),
			decayFactorEpochsSumExponent,
		),
		iotago.WithStakingOptions(10, 10, 10),
		iotago.WithLivenessOptions(15, 30, 10, 20, 24),
		iotago.WithCongestionControlOptions(500, 500, 500, 8*schedulerRate, 5*schedulerRate, schedulerRate, 1000, 100),
	)

	TestAPI = iotago.V3API(testProtoParams)
)

// L1Params describes parameters coming from the L1 node
type L1Params struct {
	Protocol  iotago.ProtocolParameters `json:"protocol" swagger:"required"`
	BaseToken *BaseTokenInfo            `json:"baseToken" swagger:"required"`
}

func isTestContext() bool {
	return strings.HasSuffix(os.Args[0], ".test") ||
		strings.HasSuffix(os.Args[0], ".test.exe") ||
		strings.Contains(os.Args[0], "__debug_bin")
}

var L1ForTesting = &L1Params{
	Protocol:  TestAPI.ProtocolParameters(),
	BaseToken: TestBaseToken,
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

func Storage() *iotago.StorageScoreStructure {
	return L1API().StorageScoreStructure()
}

func Protocol() iotago.ProtocolParameters {
	return L1().Protocol
}

func NetworkPrefix() iotago.NetworkPrefix {
	return Protocol().Bech32HRP()
}

func BaseToken() *BaseTokenInfo {
	return L1().BaseToken
}
