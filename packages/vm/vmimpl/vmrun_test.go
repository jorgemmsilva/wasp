package vmimpl

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	hivedb "github.com/iotaledger/hive.go/kvstore/database"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/tpkg"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/database"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/origin"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/state/indexedstore"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/coreprocessors"
	"github.com/iotaledger/wasp/packages/vm/core/migrations"
	"github.com/iotaledger/wasp/packages/vm/processors"
)

func TestNFTDepositNoIssuer(t *testing.T) {
	metadata := isc.RequestMetadata{
		TargetContract: accounts.Contract.Hname(),
		EntryPoint:     accounts.FuncDeposit.Hname(),
	}
	o := &iotago.NFTOutput{
		Amount: 100 * isc.Million,
		NFTID:  iotago.NFTID{0x1},
		Features: iotago.NFTOutputFeatures{
			&iotago.MetadataFeature{
				Entries: iotago.MetadataFeatureEntries{"": metadata.Bytes()},
			},
			&iotago.SenderFeature{
				Address: tpkg.RandEd25519Address(),
			},
		},
		ImmutableFeatures: iotago.NFTOutputImmFeatures{
			&iotago.MetadataFeature{
				Entries: iotago.MetadataFeatureEntries{"": []byte("foobar")},
			},
		},
	}

	res := simulateRunOutput(t, o)
	require.Len(t, res.RequestResults, 1)
	require.Nil(t, res.RequestResults[0].Receipt.Error)
}

func simulateRunOutput(t *testing.T, output iotago.Output) *vm.VMTaskResult {
	// setup a test DB
	chainRecordRegistryProvider, err := registry.NewChainRecordRegistryImpl("")
	require.NoError(t, err)
	chainStateDatabaseManager, err := database.NewChainStateDatabaseManager(chainRecordRegistryProvider, database.WithEngine(hivedb.EngineMapDB))
	require.NoError(t, err)
	db, mu, err := chainStateDatabaseManager.ChainStateKVStore(isc.EmptyChainID())
	require.NoError(t, err)

	// parse request from output
	outputID := iotago.OutputID{}
	req, err := isc.OnLedgerFromUTXO(output, outputID)
	require.NoError(t, err)

	// create the AO for a new chain
	chainCreator := cryptolib.KeyPairFromSeed(cryptolib.SeedFromBytes([]byte("foobar")))
	_, chainOutputs, _, err := origin.NewChainOriginTransaction(
		chainCreator,
		chainCreator.Address(),
		chainCreator.Address(),
		10*isc.Million,
		0,
		nil,
		iotago.OutputSet{
			iotago.OutputID{}: &iotago.BasicOutput{
				Amount: 1000 * isc.Million,
			},
		},
		0,
		0,
		testutil.L1API,
	)
	require.NoError(t, err)

	// create task and run it
	task := &vm.VMTask{
		Processors:           processors.MustNew(coreprocessors.NewConfigWithCoreContracts()),
		Inputs:               chainOutputs,
		Store:                indexedstore.New(state.NewStore(db, mu)),
		Requests:             []isc.Request{req},
		Timestamp:            time.Now(),
		Entropy:              [32]byte{},
		ValidatorFeeTarget:   nil,
		EstimateGasMode:      false,
		EVMTracer:            &isc.EVMTracer{},
		EnableGasBurnLogging: false,
		MigrationsOverride:   &migrations.MigrationScheme{},
		Log:                  testlogger.NewLogger(t),
	}

	origin.InitChainByAnchorOutput(task.Store, chainOutputs, testutil.L1API)

	return runTask(task)
}
