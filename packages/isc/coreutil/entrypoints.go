package coreutil

import (
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
)

type Field[T any] struct {
	Key   kv.Key
	Codec *codec.Codec[T]
}

func (f Field[T]) Encode(v T) dict.Dict {
	return dict.Dict{f.Key: f.Codec.Encode(v)}
}

func (f Field[T]) EncodeOpt(v ...T) dict.Dict {
	if len(v) == 0 {
		return nil
	}
	return f.Encode(v[0])
}

func (f Field[T]) Has(r dict.Dict) bool {
	return r[f.Key] != nil
}

func (f Field[T]) Decode(r dict.Dict, def ...T) (T, error) {
	return f.Codec.Decode(r[f.Key], def...)
}

func (f Field[T]) DecodeOpt(r dict.Dict) (ret T, err error) {
	if !f.Has(r) {
		return
	}
	return f.Decode(r)
}

type Field2[T1, T2 any] struct {
	F1 Field[T1]
	F2 Field[T2]
}

func (t Field2[T1, T2]) Encode(v1 T1, v2 T2) dict.Dict {
	d := t.F1.Encode(v1)
	d.Extend(t.F2.Encode(v2))
	return d
}

func (t Field2[T1, T2]) Decode(r dict.Dict) (r1 T1, r2 T2, err error) {
	r1, err = t.F1.Decode(r)
	if err != nil {
		return
	}
	r2, err = t.F2.Decode(r)
	return
}

func (t Field2[T1, T2]) DecodeOpt(r dict.Dict) (r1 T1, r2 T2, err error) {
	r1, err = t.F1.DecodeOpt(r)
	if err != nil {
		return
	}
	r2, err = t.F2.DecodeOpt(r)
	return
}

// EP0 is a utility type for entry points that receive 0 parameters
type EP0[S isc.SandboxBase] struct{ EntryPointInfo[S] }

func (e EP0[S]) Message() isc.Message { return e.EntryPointInfo.Message(nil) }

func NewEP0(contract *ContractInfo, name string) EP0[isc.Sandbox] {
	return EP0[isc.Sandbox]{EntryPointInfo: contract.Func(name)}
}

func NewViewEP0(contract *ContractInfo, name string) EP0[isc.SandboxView] {
	return EP0[isc.SandboxView]{EntryPointInfo: contract.ViewFunc(name)}
}

// EP1 is a utility type for entry points that receive 1 parameter
type EP1[S isc.SandboxBase, T any] struct {
	EntryPointInfo[S]
	Input Field[T]
}

func (e EP1[S, T]) Message(p1 T) isc.Message {
	return e.EntryPointInfo.Message(e.Input.Encode(p1))
}

// MessageOpt constructs a Message where the parameter is optional
func (e EP1[S, T]) MessageOpt(p ...T) isc.Message {
	return e.EntryPointInfo.Message(e.Input.EncodeOpt(p...))
}

func NewEP1[T any](contract *ContractInfo, name string, p1Key kv.Key, p1Codec *codec.Codec[T]) EP1[isc.Sandbox, T] {
	return EP1[isc.Sandbox, T]{
		EntryPointInfo: contract.Func(name),
		Input:          Field[T]{Key: p1Key, Codec: p1Codec},
	}
}

func NewViewEP1[T any](contract *ContractInfo, name string, p1Key kv.Key, p1Codec *codec.Codec[T]) EP1[isc.SandboxView, T] {
	return EP1[isc.SandboxView, T]{
		EntryPointInfo: contract.ViewFunc(name),
		Input:          Field[T]{Key: p1Key, Codec: p1Codec},
	}
}

// EP2 is a utility type for entry points that receive 2 parameters
type EP2[S isc.SandboxBase, T1, T2 any] struct {
	EntryPointInfo[S]
	Input Field2[T1, T2]
}

func NewEP2[T1 any, T2 any](
	contract *ContractInfo, name string,
	p1Key kv.Key, p1Codec *codec.Codec[T1],
	p2Key kv.Key, p2Codec *codec.Codec[T2],
) EP2[isc.Sandbox, T1, T2] {
	return EP2[isc.Sandbox, T1, T2]{
		EntryPointInfo: contract.Func(name),
		Input: Field2[T1, T2]{
			F1: Field[T1]{Key: p1Key, Codec: p1Codec},
			F2: Field[T2]{Key: p2Key, Codec: p2Codec},
		},
	}
}

func NewViewEP2[T1 any, T2 any](
	contract *ContractInfo, name string,
	p1Key kv.Key, p1Codec *codec.Codec[T1],
	p2Key kv.Key, p2Codec *codec.Codec[T2],
) EP2[isc.SandboxView, T1, T2] {
	return EP2[isc.SandboxView, T1, T2]{
		EntryPointInfo: contract.ViewFunc(name),
		Input: Field2[T1, T2]{
			F1: Field[T1]{Key: p1Key, Codec: p1Codec},
			F2: Field[T2]{Key: p2Key, Codec: p2Codec},
		},
	}
}

func (e EP2[S, T1, T2]) MessageOpt1(p1 T1) isc.Message {
	return e.EntryPointInfo.Message(e.Input.F1.Encode(p1))
}

func (e EP2[S, T1, T2]) MessageOpt2(p2 T2) isc.Message {
	return e.EntryPointInfo.Message(e.Input.F2.Encode(p2))
}

func (e EP2[S, T1, T2]) Message(p1 T1, p2 T2) isc.Message {
	return e.EntryPointInfo.Message(e.Input.Encode(p1, p2))
}

// EP01 is a utility type for entry points that receive 0 parameters and return 1 value
type EP01[S isc.SandboxBase, R any] struct {
	EP0[S]
	Output Field[R]
}

func NewViewEP01[R any](
	contract *ContractInfo, name string,
	r1Key kv.Key, r1Codec *codec.Codec[R],
) EP01[isc.SandboxView, R] {
	return EP01[isc.SandboxView, R]{
		EP0:    NewViewEP0(contract, name),
		Output: Field[R]{Key: r1Key, Codec: r1Codec},
	}
}

// EP02 is a utility type for entry points that receive 0 parameters and return 1 value
type EP02[S isc.SandboxBase, R1, R2 any] struct {
	EP0[S]
	Output Field2[R1, R2]
}

func NewViewEP02[R1, R2 any](
	contract *ContractInfo, name string,
	r1Key kv.Key, r1Codec *codec.Codec[R1],
	r2Key kv.Key, r2Codec *codec.Codec[R2],
) EP02[isc.SandboxView, R1, R2] {
	return EP02[isc.SandboxView, R1, R2]{
		EP0: NewViewEP0(contract, name),
		Output: Field2[R1, R2]{
			F1: Field[R1]{Key: r1Key, Codec: r1Codec},
			F2: Field[R2]{Key: r2Key, Codec: r2Codec},
		},
	}
}

// EP11 is a utility type for entry points that receive 1 parameter and return 1 value
type EP11[S isc.SandboxView, T any, R any] struct {
	EP1[S, T]
	Output Field[R]
}

func NewViewEP11[T any, R any](
	contract *ContractInfo, name string,
	p1Key kv.Key, p1Codec *codec.Codec[T],
	r1Key kv.Key, r1Codec *codec.Codec[R],
) EP11[isc.SandboxView, T, R] {
	return EP11[isc.SandboxView, T, R]{
		EP1:    NewViewEP1(contract, name, p1Key, p1Codec),
		Output: Field[R]{Key: r1Key, Codec: r1Codec},
	}
}

// EP12 is a utility type for entry points that receive 1 parameter and return 1 value
type EP12[S isc.SandboxBase, T any, R1 any, R2 any] struct {
	EP1[S, T]
	Output Field2[R1, R2]
}

func NewViewEP12[T any, R1 any, R2 any](
	contract *ContractInfo, name string,
	p1Key kv.Key, p1Codec *codec.Codec[T],
	r1Key kv.Key, r1Codec *codec.Codec[R1],
	r2Key kv.Key, r2Codec *codec.Codec[R2],
) EP12[isc.SandboxView, T, R1, R2] {
	return EP12[isc.SandboxView, T, R1, R2]{
		EP1: NewViewEP1(contract, name, p1Key, p1Codec),
		Output: Field2[R1, R2]{
			F1: Field[R1]{Key: r1Key, Codec: r1Codec},
			F2: Field[R2]{Key: r2Key, Codec: r2Codec},
		},
	}
}

// EP21 is a utility type for entry points that receive 2 parameters and return 1 value
type EP21[S isc.SandboxBase, T1 any, T2 any, R any] struct {
	EP2[S, T1, T2]
	Output Field[R]
}

func NewViewEP21[T1 any, T2 any, R any](
	contract *ContractInfo, name string,
	p1Key kv.Key, p1Codec *codec.Codec[T1],
	p2Key kv.Key, p2Codec *codec.Codec[T2],
	r1Key kv.Key, r1Codec *codec.Codec[R],
) EP21[isc.SandboxView, T1, T2, R] {
	return EP21[isc.SandboxView, T1, T2, R]{
		EP2:    NewViewEP2(contract, name, p1Key, p1Codec, p2Key, p2Codec),
		Output: Field[R]{Key: r1Key, Codec: r1Codec},
	}
}
