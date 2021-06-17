// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package wasmhost

import (
	"encoding/binary"
	"fmt"
	"time"
)

const (
	defaultTimeout     = 5 * time.Second
	disableWasmTimeout = false
)

// WasmTimeout set this to non-zero for a one-time override of the defaultTimeout
var WasmTimeout = 0 * time.Second

type WasmVM interface {
	Interrupt()
	LinkHost(impl WasmVM, host *WasmHost) error
	LoadWasm(wasmData []byte) error
	RunFunction(functionName string, args ...interface{}) error
	RunScFunction(index int32) error
	SaveMemory()
	UnsafeMemory() []byte
	VMGetBytes(offset int32, size int32) []byte
	VMSetBytes(offset int32, size int32, bytes []byte) int32
}

type WasmVMBase struct {
	impl           WasmVM
	host           *WasmHost
	memoryCopy     []byte
	memoryDirty    bool
	memoryNonZero  int
	result         []byte
	resultKeyID    int32
	timeoutStarted bool
}

func (vm *WasmVMBase) LinkHost(impl WasmVM, host *WasmHost) error {
	// trick vm into thinking it doesn't have to start the timeout timer
	// useful when debugging to prevent timing out on breakpoints
	vm.timeoutStarted = disableWasmTimeout

	vm.impl = impl
	vm.host = host
	host.vm = impl
	return nil
}

func (vm *WasmVMBase) HostFdWrite(fd, iovs, size, written int32) int32 {
	vm.host.TraceAllf("HostFdWrite(...)")
	// very basic implementation that expects fd to be stdout and iovs to be only one element
	ptr := vm.impl.UnsafeMemory()
	txt := binary.LittleEndian.Uint32(ptr[iovs : iovs+4])
	siz := binary.LittleEndian.Uint32(ptr[iovs+4 : iovs+8])
	fmt.Print(string(ptr[txt : txt+siz]))
	binary.LittleEndian.PutUint32(ptr[written:written+4], siz)
	return int32(siz)
}

func (vm *WasmVMBase) HostGetBytes(objID, keyID, typeID, stringRef, size int32) int32 {
	host := vm.host
	host.TraceAllf("HostGetBytes(o%d,k%d,t%d,r%d,s%d)", objID, keyID, typeID, stringRef, size)

	// only check for existence ?
	if size < 0 {
		if host.Exists(objID, keyID, typeID) {
			return 0
		}
		// missing key is indicated by -1
		return -1
	}

	// actual GetBytes request ?
	if (typeID & OBJTYPE_CALL) == 0 {
		bytes := host.GetBytes(objID, keyID, typeID)
		if bytes == nil {
			return -1
		}
		return vm.impl.VMSetBytes(stringRef, size, bytes)
	}

	// func call request
	switch typeID {
	case OBJTYPE_CALL:
		// func call with params, returns result length
		vm.resultKeyID = keyID
		params := vm.impl.VMGetBytes(stringRef, size)
		vm.result = host.CallFunc(objID, keyID, params)
		return int32(len(vm.result))

	case OBJTYPE_CALL + 1:
		// retrieve previous func call result
		if vm.resultKeyID == keyID {
			result := vm.result
			vm.result = nil
			vm.resultKeyID = 0
			if result == nil {
				return -1
			}
			return vm.impl.VMSetBytes(stringRef, int32(len(result)), result)
		}
	}
	panic("HostGetBytes: Invalid func call state")
}

func (vm *WasmVMBase) HostGetKeyID(keyRef, size int32) int32 {
	host := vm.host
	host.TraceAllf("HostGetKeyId(r%d,s%d)", keyRef, size)
	// non-negative size means original key was a string
	if size >= 0 {
		bytes := vm.impl.VMGetBytes(keyRef, size)
		return host.GetKeyIDFromString(string(bytes))
	}

	// negative size means original key was a byte slice
	bytes := vm.impl.VMGetBytes(keyRef, -size-1)
	return host.GetKeyIDFromBytes(bytes)
}

func (vm *WasmVMBase) hostGetObjectID(objID, keyID, typeID int32) int32 {
	host := vm.host
	host.TraceAllf("hostGetObjectID(o%d,k%d,t%d)", objID, keyID, typeID)
	return host.GetObjectID(objID, keyID, typeID)
}

func (vm *WasmVMBase) HostSetBytes(objID, keyID, typeID, stringRef, size int32) {
	host := vm.host
	host.TraceAllf("HostSetBytes(o%d,k%d,t%d,r%d,s%d)", objID, keyID, typeID, stringRef, size)
	bytes := vm.impl.VMGetBytes(stringRef, size)
	host.SetBytes(objID, keyID, typeID, bytes)
}

func (vm *WasmVMBase) PreCall() []byte {
	ptr := vm.impl.UnsafeMemory()
	frame := make([]byte, len(ptr))
	copy(frame, ptr)
	if vm.memoryDirty {
		// clear memory and restore initialized data range
		copy(ptr, make([]byte, len(ptr)))
		copy(ptr[vm.memoryNonZero:], vm.memoryCopy)
	}
	vm.memoryDirty = true
	return frame
}

func (vm *WasmVMBase) PostCall(frame []byte) {
	ptr := vm.impl.UnsafeMemory()
	copy(ptr, frame)
}

func (vm *WasmVMBase) Run(runner func() error) (err error) {
	if vm.timeoutStarted {
		// no need to wrap nested calls in timeout code
		return runner()
	}

	timeout := defaultTimeout
	if WasmTimeout != 0 {
		timeout = WasmTimeout
		WasmTimeout = 0
	}

	done := make(chan bool, 2)

	// start timeout handler
	go func() {
		select {
		case <-done: // runner was done before timeout
		case <-time.After(timeout):
			// timeout: interrupt Wasm
			vm.impl.Interrupt()
			// wait for runner to finish
			<-done
		}
	}()

	vm.timeoutStarted = true
	err = runner()
	done <- true
	vm.timeoutStarted = false
	return err
}

func (vm *WasmVMBase) SaveMemory() {
	// find initialized data range in memory
	ptr := vm.impl.UnsafeMemory()
	if ptr == nil {
		// this vm implementation does not communicate via mem pool
		return
	}
	firstNonZero := -1
	lastNonZero := 0
	for i, b := range ptr {
		if b != 0 {
			if firstNonZero < 0 {
				firstNonZero = i
			}
			lastNonZero = i
		}
	}

	// save copy of initialized data range
	vm.memoryNonZero = len(ptr)
	if firstNonZero >= 0 {
		vm.memoryNonZero = firstNonZero
		size := lastNonZero + 1 - firstNonZero
		vm.memoryCopy = make([]byte, size)
		copy(vm.memoryCopy, ptr[vm.memoryNonZero:])
	}
}

func (vm *WasmVMBase) VMGetBytes(offset, size int32) []byte {
	ptr := vm.impl.UnsafeMemory()
	bytes := make([]byte, size)
	copy(bytes, ptr[offset:offset+size])
	return bytes
}

func (vm *WasmVMBase) VMSetBytes(offset, size int32, bytes []byte) int32 {
	if size != 0 {
		ptr := vm.impl.UnsafeMemory()
		copy(ptr[offset:offset+size], bytes)
	}
	return int32(len(bytes))
}
