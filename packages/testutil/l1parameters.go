package testutil

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
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
	testProtoParams                  = iotago.NewV3SnapshotProtocolParameters(
		iotago.WithNetworkOptions("test", "test"),
		iotago.WithStorageOptions(100, 1, 10, 100, 100, 100),
		iotago.WithWorkScoreOptions(0, 1, 0, 0, 0, 0, 0, 0, 0, 0),
		// note: genesistimestamp must be zero to prevent problems with Solo timestamp
		iotago.WithTimeProviderOptions(0, 0, 10, 13),
		iotago.WithLivenessOptions(15, 30, 10, 20, 60),
		iotago.WithSupplyOptions(1813620509061365, 63, 1, 17, 32, 21, 70),
		iotago.WithCongestionControlOptions(1, 0, 0, 800_000, 500_000, 100_000, 1000, 100),
		iotago.WithStakingOptions(10, 10, 10),
		iotago.WithVersionSignalingOptions(7, 5, 7),
		iotago.WithRewardsOptions(8, 8, 11, 2, 1, 384),
		iotago.WithTargetCommitteeSize(32),
	)

	L1API         = iotago.V3API(testProtoParams)
	L1APIProvider = iotago.SingleVersionProvider(L1API)
)
