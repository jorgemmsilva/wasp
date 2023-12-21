package root

import (
	"github.com/samber/lo"

	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
)

func SetSchemaVersion(state kv.KVStore, v uint32) {
	state.Set(VarSchemaVersion, codec.Uint32.Encode(v))
}

func GetSchemaVersion(state kv.KVStoreReader) uint32 {
	return lo.Must(codec.Uint32.Decode(state.Get(VarSchemaVersion), 0))
}
