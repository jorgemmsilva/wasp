package dbmanager

import (
	"fmt"
	"os"
	"sync"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/logger"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/database/registrykvstore"
	"github.com/iotaledger/wasp/packages/database/textdb"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/registry"
)

type ChainKVStoreProvider func(chainID *iscp.ChainID) kvstore.KVStore

type DBManager struct {
	log           *logger.Logger
	registryDB    DB
	registryStore kvstore.KVStore
	databases     map[*iotago.AliasID]DB
	stores        map[*iotago.AliasID]kvstore.KVStore
	mutex         sync.RWMutex
	engine        string
}

func NewDBManager(log *logger.Logger, dbEngine string, registryConfig *registry.Config) *DBManager {
	dbm := DBManager{
		log:       log,
		databases: make(map[*iotago.AliasID]DB),
		stores:    make(map[*iotago.AliasID]kvstore.KVStore),
		mutex:     sync.RWMutex{},
		engine:    dbEngine,
	}
	// registry db is created with an empty chainID
	dbm.registryDB = dbm.createDB(nil)
	if registryConfig.UseText {
		dbm.registryStore = registrykvstore.New(textdb.NewTextKV(log, registryConfig.Filename))
	} else {
		dbm.registryStore = registrykvstore.New(dbm.registryDB.NewStore())
	}
	return &dbm
}

func getChainString(chainID *iscp.ChainID) string {
	if chainID != nil {
		return chainID.String()
	}
	return "CHAIN_REGISTRY"
}

func (m *DBManager) createDB(chainID *iscp.ChainID) DB {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	chainIDStr := getChainString(chainID)

	dbDir := parameters.GetString(parameters.DatabaseDir)
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		// create a new database dir if none exists
		err := os.Mkdir(dbDir, os.ModePerm)
		if err != nil {
			m.log.Fatal(err)
			return nil
		}
	}

	instanceDir := fmt.Sprintf("%s/%s", dbDir, chainIDStr)
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		m.log.Infof("creating new database for: %s.", chainIDStr)
	} else {
		m.log.Infof("using existing database for: %s.", chainIDStr)
	}

	db, err := NewDB(instanceDir, m.engine)
	if err != nil {
		m.log.Fatal(err)
	}
	return db
}

func (m *DBManager) GetRegistryKVStore() kvstore.KVStore {
	return m.registryStore
}

func (m *DBManager) GetOrCreateKVStore(chainID *iscp.ChainID) kvstore.KVStore {
	store := m.GetKVStore(chainID)
	if store != nil {
		return store
	}

	// create a new database / store
	db := m.createDB(chainID)
	store = db.NewStore()
	m.databases[chainID.AsAliasID()] = db
	m.stores[chainID.AsAliasID()] = db.NewStore()
	return store
}

func (m *DBManager) GetKVStore(chainID *iscp.ChainID) kvstore.KVStore {
	return m.stores[chainID.AsAliasID()]
}

func (m *DBManager) Close() {
	m.registryDB.Close()
	for _, instance := range m.databases {
		instance.Close()
	}
}
