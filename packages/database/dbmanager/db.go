package dbmanager

import (
	"fmt"

	"github.com/iotaledger/hive.go/kvstore"
)

// DB represents a database abstraction.
type DB interface {
	// NewStore creates a new KVStore backed by the database.
	NewStore() kvstore.KVStore
	// Close closes a DB.
	Close() error
}

// NewDB returns a new persisting DB object.
func NewDB(dirname, engine string) (DB, error) {
	switch engine {
	case "pebble":
		return NewPebbleDB(dirname)
	case "rocksdb":
		return NewRocksDB(dirname)
	default:
		panic(fmt.Errorf("unknown db engine: %s", engine))
	}
}
