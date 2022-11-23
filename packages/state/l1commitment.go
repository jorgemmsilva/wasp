package state

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/xerrors"

	"github.com/iotaledger/hive.go/core/marshalutil"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/trie"
	"github.com/iotaledger/wasp/packages/util"
)

const BlockHashSize = 20

type BlockHash [BlockHashSize]byte

// L1Commitment represents the data stored as metadata in the anchor output
type L1Commitment struct {
	// root commitment to the state
	TrieRoot trie.VCommitment
	// hash of the essence of the block
	BlockHash BlockHash
}

var l1CommitmentSize = trie.HashSizeBytes + BlockHashSize

func BlockHashFromData(data []byte) (ret BlockHash) {
	r := blake2b.Sum256(data)
	copy(ret[:BlockHashSize], r[:BlockHashSize])
	return
}

func newL1Commitment(c trie.VCommitment, blockHash BlockHash) *L1Commitment {
	return &L1Commitment{
		TrieRoot:  c,
		BlockHash: blockHash,
	}
}

func (bh BlockHash) String() string {
	return hexutil.Encode(bh[:])
}

func (bh BlockHash) Equals(bh2 BlockHash) bool {
	return bh == bh2
}

func L1CommitmentFromBytes(data []byte) (L1Commitment, error) {
	if len(data) != l1CommitmentSize {
		return L1Commitment{}, xerrors.New("L1CommitmentFromBytes: wrong data length")
	}
	ret := L1Commitment{}
	if err := ret.Read(bytes.NewReader(data)); err != nil {
		return L1Commitment{}, err
	}
	return ret, nil
}

func L1CommitmentFromMarshalUtil(mu *marshalutil.MarshalUtil) (*L1Commitment, error) {
	byteCount, err := mu.ReadUint16()
	if err != nil {
		return nil, err
	}
	data, err := mu.ReadBytes(int(byteCount))
	if err != nil {
		return nil, err
	}
	l1c, err := L1CommitmentFromBytes(data)
	if err != nil {
		return nil, err
	}
	return &l1c, nil
}

func L1CommitmentFromAliasOutput(output *iotago.AliasOutput) (*L1Commitment, error) {
	l1c, err := L1CommitmentFromBytes(output.StateMetadata)
	if err != nil {
		return nil, err
	}
	return &l1c, nil
}

func (s *L1Commitment) Equals(other *L1Commitment) bool {
	return s.BlockHash == other.BlockHash && s.TrieRoot.Equals(other.TrieRoot)
}

func (s *L1Commitment) Bytes() []byte {
	return util.MustBytes(s)
}

func (s *L1Commitment) Write(w io.Writer) error {
	if err := s.TrieRoot.Write(w); err != nil {
		return err
	}
	if _, err := w.Write(s.BlockHash[:]); err != nil {
		return err
	}
	return nil
}

func (s *L1Commitment) Read(r io.Reader) error {
	var err error
	s.TrieRoot, err = trie.ReadVectorCommitment(r)
	if err != nil {
		return err
	}
	if _, err := r.Read(s.BlockHash[:]); err != nil {
		return err
	}
	return nil
}

func (s *L1Commitment) String() string {
	return fmt.Sprintf("L1Commitment(%s, %s)", s.TrieRoot.String(), hexutil.Encode(s.BlockHash[:]))
}

func L1CommitmentFromAnchorOutput(o *iotago.AliasOutput) (L1Commitment, error) {
	return L1CommitmentFromBytes(o.StateMetadata)
}

var L1CommitmentNil *L1Commitment

func init() {
	zs, err := L1CommitmentFromBytes(make([]byte, l1CommitmentSize))
	if err != nil {
		panic(err)
	}
	L1CommitmentNil = &zs
}

// RandL1Commitment is for testing only
func RandL1Commitment() *L1Commitment {
	d := make([]byte, l1CommitmentSize)
	rand.Read(d)
	ret, err := L1CommitmentFromBytes(d)
	if err != nil {
		panic(err)
	}
	return &ret
}
