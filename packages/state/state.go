package state

import (
	"fmt"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/trie"
)

// state is the implementation of the State interface
type state struct {
	trieReader *trie.TrieReader
	kv.KVStoreReader
}

var _ State = &state{}

func newState(db *storeDB, root trie.Hash) (*state, error) {
	trie, err := db.trieReader(root)
	if err != nil {
		return nil, err
	}
	return &state{
		KVStoreReader: kv.NewCachedKVStoreReader(&trieKVAdapter{trie}),
		trieReader:    trie,
	}, nil
}

func (s *state) TrieRoot() trie.Hash {
	return s.trieReader.Root()
}

func (s *state) GetMerkleProof(key []byte) *trie.MerkleProof {
	return s.trieReader.MerkleProof(key)
}

func (s *state) BlockIndex() uint32 {
	return loadBlockIndexFromState(s)
}

func loadBlockIndexFromState(s kv.KVStoreReader) uint32 {
	return codec.MustDecodeUint32(s.Get(kv.Key(coreutil.StatePrefixBlockIndex)))
}

func (s *state) BlockTime() isc.BlockTime {
	t, err := loadBlockTimeFromState(s)
	mustNoErr(err)
	return t
}

func loadBlockTimeFromState(chainState kv.KVStoreReader) (ret isc.BlockTime, err error) {
	tsBin := chainState.Get(kv.Key(coreutil.StatePrefixTimestamp))
	ret.Timestamp, err = codec.DecodeTime(tsBin)
	if err != nil {
		return isc.BlockTime{}, fmt.Errorf("loadTimestampFromState: %w", err)
	}
	siBin := chainState.Get(kv.Key(coreutil.StatePrefixSlotIndex))
	if len(siBin) == 0 { // state is before 2.0 migration
		return ret, nil
	}
	ret.SlotIndex, _, err = iotago.SlotIndexFromBytes(siBin)
	return ret, err
}

func (s *state) PreviousL1Commitment() *L1Commitment {
	return loadPrevL1CommitmentFromState(s)
}

func loadPrevL1CommitmentFromState(chainState kv.KVStoreReader) *L1Commitment {
	data := chainState.Get(kv.Key(coreutil.StatePrefixPrevL1Commitment))
	l1c, err := L1CommitmentFromBytes(data)
	mustNoErr(err)
	return l1c
}

func (s *state) String() string {
	return fmt.Sprintf("State[si#%v]%v", s.BlockIndex(), s.TrieRoot())
}
