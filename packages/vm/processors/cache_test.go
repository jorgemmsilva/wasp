package processors

import (
	"testing"

	"github.com/iotaledger/wasp/packages/coretypes"
	"github.com/iotaledger/wasp/packages/coretypes/coreutil"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/packages/vm/vmtypes"
	"github.com/stretchr/testify/assert"
)

func TestBasic(t *testing.T) {
	p := MustNew(NewConfig())

	rec := root.NewContractRecord(root.Interface, &coretypes.AgentID{})
	rootproc, err := p.GetOrCreateProcessor(
		rec,
		func(hashing.HashValue) (string, []byte, error) { return vmtypes.Core, nil, nil },
	)
	assert.NoError(t, err)

	// TODO always exists because it returns default handler
	ep, exists := rootproc.GetEntryPoint(0)
	assert.True(t, exists)
	assert.Equal(t, ep.(*coreutil.ContractFunctionInterface).Name, coreutil.DefaultHandler)

	ep, exists = rootproc.GetEntryPoint(coretypes.Hn(root.FuncDeployContract))
	assert.True(t, exists)
	assert.Equal(t, ep.(*coreutil.ContractFunctionInterface).Name, root.FuncDeployContract)
}
