package testutil

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
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

var (
	TokenInfo = &api.InfoResBaseToken{
		Name:         "TestCoin",
		TickerSymbol: "TEST",
		Unit:         "TEST",
		Subunit:      "testies",
		Decimals:     6,
	}

	schedulerRate   iotago.WorkScore = 100000
	testProtoParams                  = iotago.NewV3ProtocolParameters(
		iotago.WithNetworkOptions("test", "test"),
		iotago.WithSupplyOptions(tpkg.TestTokenSupply, 100, 1, 10, 100, 100, 100),
		iotago.WithWorkScoreOptions(1, 100, 20, 20, 20, 20, 100, 100, 100, 200),
		iotago.WithTimeProviderOptions(0, 100, slotDurationSeconds, slotsPerEpochExponent),
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

	L1API         = iotago.V3API(testProtoParams)
	L1APIProvider = iotago.SingleVersionProvider(L1API)
)
