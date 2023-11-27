package accounts

import (
	"bytes"
	"io"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

// foundryOutputRec contains information to reconstruct output
type foundryOutputRec struct {
	OutputID    iotago.OutputID
	Amount      iotago.BaseToken // always storage deposit
	TokenScheme iotago.TokenScheme
	Metadata    []byte
}

func (rec *foundryOutputRec) Bytes(l1API iotago.API) []byte {
	buf := bytes.NewBuffer([]byte{})
	err := rec.Write(buf, l1API)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func foundryOutputRecFromBytes(data []byte, l1API iotago.API) (*foundryOutputRec, error) {
	f := new(foundryOutputRec)
	err := f.Read(bytes.NewBuffer(data), l1API)
	return f, err
}

func mustFoundryOutputRecFromBytes(data []byte, l1API iotago.API) *foundryOutputRec {
	ret, err := foundryOutputRecFromBytes(data, l1API)
	if err != nil {
		panic(err)
	}
	return ret
}

func (rec *foundryOutputRec) Read(r io.Reader, l1API iotago.API) error {
	rr := rwutil.NewReader(r)
	rr.ReadN(rec.OutputID[:])
	rec.Amount = iotago.BaseToken(rr.ReadUint64())
	tokenScheme := rr.ReadBytes()
	if rr.Err == nil {
		rec.TokenScheme, rr.Err = codec.DecodeTokenScheme(tokenScheme, l1API)
	}
	rec.Metadata = rr.ReadBytes()
	return rr.Err
}

func (rec *foundryOutputRec) Write(w io.Writer, l1API iotago.API) error {
	ww := rwutil.NewWriter(w)
	ww.WriteN(rec.OutputID[:])
	ww.WriteUint64(uint64(rec.Amount))
	if ww.Err == nil {
		tokenScheme := codec.EncodeTokenScheme(rec.TokenScheme, l1API)
		ww.WriteBytes(tokenScheme)
	}
	ww.WriteBytes(rec.Metadata)
	return ww.Err
}
