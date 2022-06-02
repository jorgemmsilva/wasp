package dbmanager

import (
	"github.com/cockroachdb/pebble"
	"github.com/iotaledger/hive.go/kvstore"
	hive_pebble "github.com/iotaledger/hive.go/kvstore/pebble"
)

type PebbleDB struct {
	*pebble.DB
}

var _ DB = &PebbleDB{}

func NewPebbleDB(dirname string) (*PebbleDB, error) {
	db, err := hive_pebble.CreateDB(dirname)
	return &PebbleDB{db}, err
}

func (db *PebbleDB) NewStore() kvstore.KVStore {
	return hive_pebble.New(db.DB)
}

// Close closes a DB. It's crucial to call it to ensure all the pending updates make their way to disk.
func (db *PebbleDB) Close() error {
	return db.Close()
}
