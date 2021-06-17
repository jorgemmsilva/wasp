// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package wasmhost

import (
	"errors"

	"github.com/bytecodealliance/wasmtime-go"
)

type WasmTimeVM struct {
	WasmVMBase
	instance  *wasmtime.Instance
	interrupt *wasmtime.InterruptHandle
	linker    *wasmtime.Linker
	memory    *wasmtime.Memory
	module    *wasmtime.Module
	store     *wasmtime.Store
}

var _ WasmVM = &WasmTimeVM{}

func NewWasmTimeVM() *WasmTimeVM {
	vm := &WasmTimeVM{}
	config := wasmtime.NewConfig()
	config.SetInterruptable(true)
	vm.store = wasmtime.NewStore(wasmtime.NewEngineWithConfig(config))
	vm.interrupt, _ = vm.store.InterruptHandle()
	vm.linker = wasmtime.NewLinker(vm.store)
	return vm
}

func (vm *WasmTimeVM) Interrupt() {
	vm.interrupt.Interrupt()
}

func (vm *WasmTimeVM) LinkHost(impl WasmVM, host *WasmHost) error {
	_ = vm.WasmVMBase.LinkHost(impl, host)

	err := vm.linker.DefineFunc("WasmLib", "hostGetBytes",
		func(objId int32, keyId int32, typeId int32, stringRef int32, size int32) int32 {
			return vm.HostGetBytes(objId, keyId, typeId, stringRef, size)
		})
	if err != nil {
		return err
	}
	err = vm.linker.DefineFunc("WasmLib", "hostGetKeyId",
		func(keyRef int32, size int32) int32 {
			return vm.HostGetKeyID(keyRef, size)
		})
	if err != nil {
		return err
	}
	err = vm.linker.DefineFunc("WasmLib", "hostGetObjectID",
		func(objID, keyID, typeID int32) int32 {
			return vm.hostGetObjectID(objID, keyID, typeID)
		})
	if err != nil {
		return err
	}
	err = vm.linker.DefineFunc("WasmLib", "hostSetBytes",
		func(objId int32, keyId int32, typeId int32, stringRef int32, size int32) {
			vm.HostSetBytes(objId, keyId, typeId, stringRef, size)
		})
	if err != nil {
		return err
	}

	// TinyGo Wasm versions uses this one to write panic message to console
	fdWrite := func(fd int32, iovs int32, size int32, written int32) int32 {
		return vm.HostFdWrite(fd, iovs, size, written)
	}
	err = vm.linker.DefineFunc("wasi_unstable", "fd_write", fdWrite)
	if err != nil {
		return err
	}
	return vm.linker.DefineFunc("wasi_snapshot_preview1", "fd_write", fdWrite)
}

func (vm *WasmTimeVM) LoadWasm(wasmData []byte) error {
	var err error
	vm.module, err = wasmtime.NewModule(vm.store.Engine, wasmData)
	if err != nil {
		return err
	}
	vm.instance, err = vm.linker.Instantiate(vm.module)
	if err != nil {
		return err
	}
	memory := vm.instance.GetExport("memory")
	if memory == nil {
		return errors.New("no memory export")
	}
	vm.memory = memory.Memory()
	if vm.memory == nil {
		return errors.New("not a memory type")
	}
	return nil
}

func (vm *WasmTimeVM) RunFunction(functionName string, args ...interface{}) error {
	export := vm.instance.GetExport(functionName)
	if export == nil {
		return errors.New("unknown export function: '" + functionName + "'")
	}
	return vm.Run(func() (err error) {
		_, err = export.Func().Call(args...)
		return
	})
}

func (vm *WasmTimeVM) RunScFunction(index int32) error {
	export := vm.instance.GetExport("on_call")
	if export == nil {
		return errors.New("unknown export function: 'on_call'")
	}
	frame := vm.PreCall()
	err := vm.Run(func() (err error) {
		_, err = export.Func().Call(index)
		return
	})
	vm.PostCall(frame)
	return err
}

func (vm *WasmTimeVM) UnsafeMemory() []byte {
	return vm.memory.UnsafeData()
}
