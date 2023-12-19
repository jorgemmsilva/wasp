package transaction

import (
	"errors"
	"fmt"
	"io"

	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/util/rwutil"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

const (
	// L1Commitment calculation has changed from version 0 to version 1.
	// The structure is actually the same, but the L1 commitment in V0
	// refers to an empty state, and in V1 refers to the first initialized
	// state.
	StateMetadataSupportedVersion byte = 1

	MaxStateMetadataSize   = iotago.MaxPayloadSize
	StateMetadataFixedSize = serializer.OneByte + serializer.UInt32ByteSize + state.L1CommitmentSize + gas.FeePolicyByteSize + serializer.UInt16ByteSize + serializer.UInt32ByteSize
	MaxPublicURLLength     = MaxStateMetadataSize - StateMetadataFixedSize
)

type StateMetadata struct {
	Version       byte
	L1Commitment  *state.L1Commitment
	GasFeePolicy  *gas.FeePolicy
	SchemaVersion uint32
	PublicURL     string
}

func NewStateMetadata(
	l1Commitment *state.L1Commitment,
	gasFeePolicy *gas.FeePolicy,
	schemaVersion uint32,
	publicURL string,
) *StateMetadata {
	return &StateMetadata{
		Version:       StateMetadataSupportedVersion,
		L1Commitment:  l1Commitment,
		GasFeePolicy:  gasFeePolicy,
		SchemaVersion: schemaVersion,
		PublicURL:     publicURL,
	}
}

func StateMetadataFromBytes(data []byte) (*StateMetadata, error) {
	return rwutil.ReadFromBytes(data, new(StateMetadata))
}

func (s *StateMetadata) Bytes() []byte {
	return rwutil.WriteToBytes(s)
}

func (s *StateMetadata) Read(r io.Reader) error {
	rr := rwutil.NewReader(r)
	s.Version = rr.ReadByte()
	if s.Version > StateMetadataSupportedVersion && rr.Err == nil {
		return fmt.Errorf("unsupported state metadata version: %d", s.Version)
	}
	s.SchemaVersion = rr.ReadUint32()
	s.L1Commitment = new(state.L1Commitment)
	rr.Read(s.L1Commitment)
	s.GasFeePolicy = new(gas.FeePolicy)
	rr.Read(s.GasFeePolicy)
	s.PublicURL = rr.ReadString()
	return rr.Err
}

func (s *StateMetadata) Write(w io.Writer) error {
	ww := rwutil.NewWriter(w)
	ww.WriteByte(StateMetadataSupportedVersion)
	ww.WriteUint32(s.SchemaVersion)
	ww.Write(s.L1Commitment)
	ww.Write(s.GasFeePolicy)
	ww.WriteString(s.PublicURL)
	return ww.Err
}

/////////////// avoiding circular imports: state <-> transaction //////////////////

func StateMetadataBytesFromAnchorOutput(ao *iotago.AnchorOutput) ([]byte, error) {
	if ao.FeatureSet().StateMetadata() == nil {
		return nil, errors.New("missing StateMetadata feature in AnchorOutput")
	}
	r := ao.FeatureSet().StateMetadata().Entries[""]
	if len(r) == 0 {
		return nil, errors.New("missing StateMetadata feature in AnchorOutput")
	}
	return r, nil
}

func StateMetadataFromAnchorOutput(ao *iotago.AnchorOutput) (*StateMetadata, error) {
	b, err := StateMetadataBytesFromAnchorOutput(ao)
	if err != nil {
		return nil, err
	}
	s, err := StateMetadataFromBytes(b)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func L1CommitmentFromAnchorOutput(ao *iotago.AnchorOutput) (*state.L1Commitment, error) {
	s, err := StateMetadataFromAnchorOutput(ao)
	if err != nil {
		return nil, err
	}
	return s.L1Commitment, nil
}
