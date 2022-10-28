package mempool

import (
	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
)

const (
	msgTypeShareRequest   byte = iota
	msgTypeMissingRequest byte = iota
)

// share offledger req
type msgShareRequest struct {
	gpa.BasicMessage
	req isc.Request
}

var _ gpa.Message = &msgShareRequest{}

func newMsgShareRequest(req isc.Request) *msgShareRequest {
	return &msgShareRequest{
		req: req,
	}
}

func (msg *msgShareRequest) MarshalBinary() (data []byte, err error) {
	ret := []byte{msgTypeMissingRequest}
	ret = append(ret, msg.req.Bytes()...)
	return ret, nil
}

func (msg *msgShareRequest) UnmarshalBinary(data []byte) (err error) {
	msg.req, err = isc.NewRequestFromBytes(data)
	return err
}

// ----------------------------------------------------------------

// ask for missing req
type msgMissingRequest struct {
	gpa.BasicMessage
	ref *isc.RequestRef
}

var _ gpa.Message = &msgMissingRequest{}

func newMsgMissingRequest(ref *isc.RequestRef) *msgMissingRequest {
	return &msgMissingRequest{
		ref: ref,
	}
}

func (msg *msgMissingRequest) MarshalBinary() (data []byte, err error) {
	ret := []byte{msgTypeMissingRequest}
	ret = append(ret, msg.ref.Bytes()...)
	return ret, nil
}

func (msg *msgMissingRequest) UnmarshalBinary(data []byte) (err error) {
	msg.ref, err = isc.RequestRefFromBytes(data[1:])
	return err
}
