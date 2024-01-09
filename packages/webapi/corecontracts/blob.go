package corecontracts

import (
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/vm/core/blob"
)

func GetBlobInfo(callViewInvoker CallViewInvoker, blobHash hashing.HashValue, blockIndexOrTrieRoot string) (map[string]uint32, bool, error) {
	_, ret, err := callViewInvoker(blob.ViewGetBlobInfo.Message(blobHash), blockIndexOrTrieRoot)
	if err != nil {
		return nil, false, err
	}
	fields, err := blob.ViewGetBlobInfo.Output.Decode(ret)
	return fields, len(fields) > 0, err
}

func GetBlobValue(callViewInvoker CallViewInvoker, blobHash hashing.HashValue, key string, blockIndexOrTrieRoot string) ([]byte, error) {
	_, ret, err := callViewInvoker(
		blob.ViewGetBlobField.Message(blobHash, []byte(key)),
		blockIndexOrTrieRoot,
	)
	if err != nil {
		return nil, err
	}
	return blob.ViewGetBlobField.Output.Decode(ret)
}

func ListBlobs(callViewInvoker CallViewInvoker, blockIndexOrTrieRoot string) (map[hashing.HashValue]uint32, error) {
	_, ret, err := callViewInvoker(blob.ViewListBlobs.Message(), blockIndexOrTrieRoot)
	if err != nil {
		return nil, err
	}
	return blob.ViewListBlobs.Output.Decode(ret)
}
