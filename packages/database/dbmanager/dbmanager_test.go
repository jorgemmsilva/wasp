package dbmanager

import (
	"os"
	"testing"

	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
	"github.com/stretchr/testify/require"
)

func testDB(t *testing.T, engine string) {
	parameters.Init().Set(parameters.DatabaseDir, os.TempDir())
	log := testlogger.NewLogger(t)
	dbm := NewDBManager(log, engine, registry.DefaultConfig())
	chainID := iscp.RandomChainID()
	require.Nil(t, dbm.GetKVStore(chainID))
	require.NotNil(t, dbm.GetOrCreateKVStore(chainID))
	require.Len(t, dbm.databases, 1)
	require.Len(t, dbm.stores, 1)
}

func TestCreateDbPebble(t *testing.T) {
	testDB(t, "pebble")
}
