package parameters

import (
	"os"
	"strings"
	"sync"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
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

	TestAPI         = iotago.V3API(testProtoParams)
	TestAPIProvider = api.SingleVersionProvider(TestAPI)
)

type L1ProviderParams struct {
	iotago.APIProvider
	BaseToken *BaseTokenInfo
}

// L1Params describes parameters coming from the L1 node at a given epoch
type L1Params struct {
	Protocol  iotago.ProtocolParameters `json:"protocol" swagger:"required"`
	BaseToken *BaseTokenInfo            `json:"baseToken" swagger:"required"`
}

func isTestContext() bool {
	return strings.HasSuffix(os.Args[0], ".test") ||
		strings.HasSuffix(os.Args[0], ".test.exe") ||
		strings.Contains(os.Args[0], "__debug_bin") ||
		strings.Contains(os.Args[0], "debug.test")
}

var L1Provider = sync.OnceValue(func() *L1ProviderParams {
	if !isTestContext() {
		panic("call InitL1() first")
	}
	return &L1ProviderParams{
		APIProvider: TestAPIProvider,
		BaseToken:   TestBaseToken,
	}
})

// InitL1Lazy sets a function to be called the first time L1() is called.
func InitL1Lazy(f func() *L1ProviderParams) {
	L1Provider = sync.OnceValue(f)
}

func InitL1(l1 *L1ProviderParams) {
	L1Provider = func() *L1ProviderParams {
		return l1
	}
}

func L1() *L1Params {
	provider := L1Provider()
	return &L1Params{
		Protocol:  provider.LatestAPI().ProtocolParameters(),
		BaseToken: provider.BaseToken,
	}
}

func L1API() iotago.API {
	return L1Provider().LatestAPI()
}

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
