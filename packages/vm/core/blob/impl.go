package blob

import (
	"fmt"

	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
)

var Processor = Contract.Processor(initialize,
	FuncStoreBlob.WithHandler(storeBlob),
	ViewGetBlobField.WithHandler(getBlobField),
	ViewGetBlobInfo.WithHandler(getBlobInfo),
	ViewListBlobs.WithHandler(listBlobs),
)

func initialize(ctx isc.Sandbox) []byte {
	// storing hname as a terminal value of the contract's state root.
	// This way we will be able to retrieve commitment to the contract's state
	ctx.State().Set("", ctx.Contract().Bytes())
	return nil
}

// storeBlob treats parameters as names of fields and field values
// it stores it in the state in deterministic binary representation
// Returns hash of the blob
func storeBlob(ctx isc.Sandbox) []byte {
	ctx.Log().Debugf("blob.storeBlob.begin")
	state := ctx.State()
	params := ctx.Params()
	// calculate a deterministic hash of all blob fields
	blobHash, kSorted, values := mustGetBlobHash(params.Dict)

	directory := GetDirectory(state)
	ctx.Requiref(!directory.MustHasAt(blobHash[:]),
		"blob.storeBlob.fail: blob with hash %s already exists", blobHash.String())

	// get a record by blob hash
	blbValues := GetBlobValues(state, blobHash)
	blbSizes := GetBlobSizes(state, blobHash)

	totalSize := uint32(0)
	totalSizeWithKeys := uint32(0)

	// save record of the blob. In parallel save record of sizes of blob fields
	sizes := make([]uint32, len(kSorted))
	for i, k := range kSorted {
		size := uint32(len(values[i]))
		if size > getMaxBlobSize(ctx) {
			ctx.Log().Panicf("blob too big. received size: %d", size)
		}
		blbValues.MustSetAt([]byte(k), values[i])
		blbSizes.MustSetAt([]byte(k), EncodeSize(size))
		sizes[i] = size
		totalSize += size
		totalSizeWithKeys += size + uint32(len(k))
	}

	directory.MustSetAt(blobHash[:], EncodeSize(totalSize))

	ctx.Event(fmt.Sprintf("[blob] hash: %s, field sizes: %+v", blobHash.String(), sizes))
	return util.MustSerialize(blobHash)
}

// getBlobInfo return lengths of all fields in the blob
func getBlobInfo(ctx isc.SandboxView) []byte {
	ctx.Log().Debugf("blob.getBlobInfo.begin")

	blobHash := ctx.Params().MustGetHashValue(ParamHash)

	blbSizes := GetBlobSizesR(ctx.StateR(), blobHash)
	ret := make(map[string]uint32)
	var err error
	blbSizes.MustIterate(func(field []byte, value []byte) bool {
		ret[string(field)], err = DecodeSize(value)
		ctx.RequireNoError(err)
		return true
	})
	return util.MustSerialize(ret)
}

func getBlobField(ctx isc.SandboxView) []byte {
	ctx.Log().Debugf("blob.getBlobField.begin")
	state := ctx.StateR()

	blobHash := ctx.Params().MustGetHashValue(ParamHash)
	field := ctx.Params().MustGetBytes(ParamField)

	blobValues := GetBlobValuesR(state, blobHash)
	ctx.Requiref(blobValues.MustLen() != 0, "blob with hash %s has not been found", blobHash.String())
	value := blobValues.MustGetAt(field)
	ctx.Requiref(value != nil, "'blob field %s value not found", string(field))
	return value
}

func listBlobs(ctx isc.SandboxView) []byte {
	ctx.Log().Debugf("blob.listBlobs.begin")
	ret := make(map[string]uint32)
	var err error
	GetDirectoryR(ctx.StateR()).MustIterate(func(hash []byte, totalSize []byte) bool {
		ret[string(hash)], err = DecodeSize(totalSize)
		ctx.RequireNoError(err)
		return true
	})
	return util.MustSerialize(ret)
}

func getMaxBlobSize(ctx isc.Sandbox) uint32 {
	data := ctx.Call(governance.Contract.Hname(), governance.ViewGetMaxBlobSize.Hname(), nil, nil)
	return util.MustDeserialize[uint32](data)
}
