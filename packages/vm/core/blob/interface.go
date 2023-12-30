package blob

import (
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/kv/dict"
)

var Contract = coreutil.NewContract(coreutil.CoreContractBlob)

var (
	FuncStoreBlob = Contract.Func("storeBlob")

	ViewGetBlobInfo = EPViewGetBlobInfo{EP1: coreutil.NewViewEP1(Contract, "getBlobInfo",
		ParamHash, codec.HashValue,
	)}
	ViewGetBlobField = coreutil.NewViewEP21(Contract, "getBlobField",
		ParamHash, codec.HashValue,
		ParamField, codec.Bytes,
		ParamBytes, codec.Bytes,
	)
	ViewListBlobs = EPViewListBlobs{EP0: coreutil.NewViewEP0(Contract, "listBlobs")}
)

// state variables
const (
	// variable names of standard blob's field
	// user-defined field must be different
	VarFieldProgramBinary      = "p"
	VarFieldVMType             = "v"
	VarFieldProgramDescription = "d"
)

// request parameters
const (
	ParamHash  = "hash"
	ParamField = "field"
	ParamBytes = "bytes"
)

// FieldValueKey returns key of the blob field value in the SC state.
func FieldValueKey(blobHash hashing.HashValue, fieldName string) []byte {
	return []byte(collections.MapElemKey(valuesMapName(blobHash), []byte(fieldName)))
}

type EPViewGetBlobInfo struct {
	coreutil.EP1[isc.SandboxView, hashing.HashValue]
	Output FieldSizesMap
}

type FieldSizesMap struct{}

func (f FieldSizesMap) Decode(r dict.Dict) (map[string]uint32, bool, error) {
	return decodeSizesMap(r)
}

type EPViewListBlobs struct {
	coreutil.EP0[isc.SandboxView]
	Output FieldDirectory
}

type FieldDirectory struct{}

func (f FieldDirectory) Decode(r dict.Dict) (map[hashing.HashValue]uint32, error) {
	return decodeDirectory(r)
}
