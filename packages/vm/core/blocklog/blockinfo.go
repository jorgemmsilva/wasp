package blocklog

import (
	"fmt"
	"io"
	"time"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

const (
	BlockInfoLatestSchemaVersion = 1
)

type BlockInfo struct {
	SchemaVersion         uint8
	Time                  isc.BlockTime // schema version 0 -> just time.Time
	TotalRequests         uint16
	NumSuccessfulRequests uint16 // which didn't panic
	NumOffLedgerRequests  uint16
	PreviousAccountOutput *isc.AccountOutputWithID // nil for block #0
	GasBurned             uint64
	GasFeeCharged         iotago.BaseToken
}

func (bi *BlockInfo) String() string {
	ret := "{\n"
	ret += fmt.Sprintf("\tBlock index: %d\n", bi.BlockIndex())
	ret += fmt.Sprintf("\tSchemaVersion: %d\n", bi.SchemaVersion)
	ret += fmt.Sprintf("\tSlot Index: %d\n", bi.Time.SlotIndex)
	ret += fmt.Sprintf("\tTimestamp: %d\n", bi.Time.Timestamp.Unix())
	ret += fmt.Sprintf("\tTotal requests: %d\n", bi.TotalRequests)
	ret += fmt.Sprintf("\toff-ledger requests: %d\n", bi.NumOffLedgerRequests)
	ret += fmt.Sprintf("\tSuccessful requests: %d\n", bi.NumSuccessfulRequests)
	ret += fmt.Sprintf("\tPrev AccountOutput: %s\n", bi.PreviousAccountOutput.String())
	ret += fmt.Sprintf("\tGas burned: %d\n", bi.GasBurned)
	ret += fmt.Sprintf("\tGas fee charged: %d\n", bi.GasFeeCharged)
	ret += "}\n"
	return ret
}

func (bi *BlockInfo) Bytes() []byte {
	return rwutil.WriteToBytes(bi)
}

func BlockInfoFromBytes(data []byte) (*BlockInfo, error) {
	return rwutil.ReadFromBytes(data, new(BlockInfo))
}

// BlockInfoKey a key to access block info record inside SC state
func BlockInfoKey(index uint32) []byte {
	return []byte(collections.ArrayElemKey(PrefixBlockRegistry, index))
}

func (bi *BlockInfo) BlockIndex() uint32 {
	if bi.PreviousAccountOutput == nil {
		return 0
	}
	return bi.PreviousAccountOutput.GetStateIndex() + 1
}

func (bi *BlockInfo) Read(r io.Reader) error {
	rr := rwutil.NewReader(r)
	bi.SchemaVersion = rr.ReadUint8()
	if bi.SchemaVersion >= 1 {
		bi.Time.SlotIndex = iotago.SlotIndex(rr.ReadAmount64())
	}
	bi.Time.Timestamp = time.Unix(0, rr.ReadInt64())
	bi.TotalRequests = rr.ReadUint16()
	bi.NumSuccessfulRequests = rr.ReadUint16()
	bi.NumOffLedgerRequests = rr.ReadUint16()
	hasPreviousAccountOutput := rr.ReadBool()
	if hasPreviousAccountOutput {
		bi.PreviousAccountOutput = &isc.AccountOutputWithID{}
		rr.Read(bi.PreviousAccountOutput)
	}
	bi.GasBurned = rr.ReadGas64()
	bi.GasFeeCharged = iotago.BaseToken(rr.ReadGas64())
	return rr.Err
}

func (bi *BlockInfo) Write(w io.Writer) error {
	ww := rwutil.NewWriter(w)
	ww.WriteUint8(bi.SchemaVersion)
	if bi.SchemaVersion >= 1 {
		ww.WriteAmount64(uint64(bi.Time.SlotIndex))
	}
	ww.WriteInt64(bi.Time.Timestamp.UnixNano())
	ww.WriteUint16(bi.TotalRequests)
	ww.WriteUint16(bi.NumSuccessfulRequests)
	ww.WriteUint16(bi.NumOffLedgerRequests)
	ww.WriteBool(bi.PreviousAccountOutput != nil)
	if bi.PreviousAccountOutput != nil {
		ww.Write(bi.PreviousAccountOutput)
	}
	ww.WriteGas64(bi.GasBurned)
	ww.WriteGas64(uint64(bi.GasFeeCharged))
	return ww.Err
}
