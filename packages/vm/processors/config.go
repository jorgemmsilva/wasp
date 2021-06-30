package processors

import (
	"fmt"

	"github.com/iotaledger/wasp/packages/coretypes"
	"github.com/iotaledger/wasp/packages/coretypes/coreutil"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/vm/core"
	"github.com/iotaledger/wasp/packages/vm/vmtypes"
)

type VMConstructor func(binaryCode []byte) (coretypes.VMProcessor, error)

type Config struct {
	// vmConstructors is the collection of registered non-native VM types
	vmConstructors map[string]VMConstructor

	// nativeContracts is the collection of registered native contracts
	nativeContracts map[hashing.HashValue]coretypes.VMProcessor
}

func NewConfig(nativeContracts ...*coreutil.ContractInterface) *Config {
	p := &Config{
		vmConstructors:  make(map[string]VMConstructor),
		nativeContracts: make(map[hashing.HashValue]coretypes.VMProcessor),
	}
	for _, c := range nativeContracts {
		p.RegisterNativeContract(c)
	}
	return p
}

// RegisterVMType registers new VM type by providing a constructor function to construct
// an instance of the processor.
// The constructor is a closure which also may encompass configuration params for the VM
// The function is normally called from the init code
func (p *Config) RegisterVMType(vmtype string, constructor VMConstructor) error {
	if _, ok := p.vmConstructors[vmtype]; ok {
		return fmt.Errorf("duplicate vm type '%s'", vmtype)
	}
	p.vmConstructors[vmtype] = constructor
	return nil
}

// NewProcessorFromBinary creates an instance of the processor by its VM type and the binary code
func (p *Config) NewProcessorFromBinary(vmtype string, binaryCode []byte) (coretypes.VMProcessor, error) {
	constructor, ok := p.vmConstructors[vmtype]
	if !ok {
		return nil, fmt.Errorf("unknown VM type '%s'", vmtype)
	}
	return constructor(binaryCode)
}

// GetNativeProcessorType returns the type of the native processor
func (p *Config) GetNativeProcessorType(programHash hashing.HashValue) (string, bool) {
	if _, err := core.GetProcessor(programHash); err == nil {
		return vmtypes.Core, true
	}
	if _, ok := p.GetNativeProcessor(programHash); ok {
		return vmtypes.Native, true
	}
	return "", false
}

// RegisterNativeContract registers a native contract so that it may be deployed
func (p *Config) RegisterNativeContract(c *coreutil.ContractInterface) {
	p.nativeContracts[c.ProgramHash] = c
}

func (p *Config) GetNativeProcessor(programHash hashing.HashValue) (coretypes.VMProcessor, bool) {
	proc, ok := p.nativeContracts[programHash]
	return proc, ok
}
