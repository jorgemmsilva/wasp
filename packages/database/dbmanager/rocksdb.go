package dbmanager

import (
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/rocksdb"
)

type RocksDB struct {
	*rocksdb.RocksDB
}

var _ DB = &RocksDB{}

func NewRocksDB(dirname string) (*RocksDB, error) {
	db, err := rocksdb.CreateDB(dirname)
	return &RocksDB{RocksDB: db}, err
}

func (db *RocksDB) NewStore() kvstore.KVStore {
	return rocksdb.New(db.RocksDB)
}

// Close closes a DB. It's crucial to call it to ensure all the pending updates make their way to disk.
func (db *RocksDB) Close() error {
	return db.RocksDB.Close()
}
