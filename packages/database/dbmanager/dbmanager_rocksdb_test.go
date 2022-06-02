//go:build rocksdb

package dbmanager

import (
	"testing"
)

func TestCreateDbRocksdb(t *testing.T) {
	testDB(t, "rocksdb")
}
