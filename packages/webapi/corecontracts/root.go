package corecontracts

import (
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/vm/core/blob"
)

func GetBlobInfo(callViewInvoker CallViewInvoker, blobHash hashing.HashValue, blockIndexOrTrieRoot string) (map[string]uint32, bool, error) {
	_, ret, err := callViewInvoker(
		blob.Contract.Hname(),
		blob.ViewGetBlobInfo.Hname(),
		codec.MakeDict(map[string]interface{}{
			blob.ParamHash: blobHash.Bytes(),
		}),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return nil, false, err
	}

	if ret.IsEmpty() {
		return nil, false, nil
	}

	blobMap, err := blob.DecodeSizesMap(ret)
	if err != nil {
		return nil, false, err
	}

	return blobMap, true, nil
}

func GetBlobValue(callViewInvoker CallViewInvoker, blobHash hashing.HashValue, key string, blockIndexOrTrieRoot string) ([]byte, error) {
	_, ret, err := callViewInvoker(
		blob.Contract.Hname(),
		blob.ViewGetBlobField.Hname(),
		codec.MakeDict(map[string]interface{}{
			blob.ParamHash:  blobHash.Bytes(),
			blob.ParamField: []byte(key),
		}),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return nil, err
	}

	return ret[blob.ParamBytes], nil
}

func ListBlobs(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) (map[hashing.HashValue]uint32, error) {
	_, ret, err := callViewInvoker(blob.Contract.Hname(), blob.ViewListBlobs.Hname(), nil, blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}

	return blob.DecodeDirectory(ret)
}
