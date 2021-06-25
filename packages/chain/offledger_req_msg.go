package chain

import (
	"bytes"
	"io"
	"io/ioutil"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/wasp/packages/coretypes/chainid"
	"github.com/iotaledger/wasp/packages/coretypes/request"
	"golang.org/x/xerrors"
)

type OffLedgerRequestMsg struct {
	ChainID *chainid.ChainID
	Req     *request.RequestOffLedger
}

func NewOffledgerRequestMsg(chainID *chainid.ChainID, req *request.RequestOffLedger) *OffLedgerRequestMsg {
	return &OffLedgerRequestMsg{
		ChainID: chainID,
		Req:     req,
	}
}

func (msg *OffLedgerRequestMsg) write(w io.Writer) error {
	if _, err := w.Write(msg.ChainID.Bytes()); err != nil {
		return xerrors.Errorf("failed to write chainID: %w", err)
	}
	if _, err := w.Write(msg.Req.Bytes()); err != nil {
		return xerrors.Errorf("failed to write reqDuest data")
	}
	return nil
}

func (msg *OffLedgerRequestMsg) Bytes() []byte {
	var buf bytes.Buffer
	_ = msg.write(&buf)
	return buf.Bytes()
}

func (msg *OffLedgerRequestMsg) read(r io.Reader) error {
	// read chainID
	var chainIDBytes [ledgerstate.AddressLength]byte
	_, err := r.Read(chainIDBytes[:])
	if err != nil {
		return xerrors.Errorf("failed to read chainID: %w", err)
	}
	if msg.ChainID, err = chainid.ChainIDFromBytes(chainIDBytes[:]); err != nil {
		return xerrors.Errorf("failed to read chainID: %w", err)
	}
	// read off-ledger request
	reqBytes, err := ioutil.ReadAll(r)
	if err != nil {
		return xerrors.Errorf("failed to read request data: %w", err)
	}
	if msg.Req, err = request.OffLedgerFromBytes(reqBytes); err != nil {
		return xerrors.Errorf("failed to read request data: %w", err)
	}
	return nil
}

func OffLedgerRequestMsgFromBytes(buf []byte) (OffLedgerRequestMsg, error) {
	r := bytes.NewReader(buf)
	msg := OffLedgerRequestMsg{}
	err := msg.read(r)
	return msg, err
}
