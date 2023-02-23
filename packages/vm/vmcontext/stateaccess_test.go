package vmcontext

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/origin"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/vm"
)

func TestSetThenGet(t *testing.T) {
	db := mapdb.NewMapDB()
	cs := origin.InitChain(state.NewStore(db), nil, 0)
	latest, err := cs.LatestBlock()
	assert.NoError(t, err)
	stateDraft, err := cs.NewStateDraft(time.Now(), latest.L1Commitment())
	assert.NoError(t, err)

	stateUpdate := NewStateUpdate()
	hname := isc.Hn("test")

	vmctx := &VMContext{
		task:               &vm.VMTask{StateDraft: stateDraft},
		currentStateUpdate: stateUpdate,
		callStack:          []*callContext{{contract: hname}},
	}
	s := vmctx.State()

	subpartitionedKey := kv.Key(hname.Bytes()) + "x"

	// contract sets variable x
	s.Set("x", []byte{42})
	assert.Equal(t, map[kv.Key][]byte{subpartitionedKey: {42}}, stateUpdate.Mutations.Sets)
	assert.Equal(t, map[kv.Key]struct{}{}, stateUpdate.Mutations.Dels)

	// contract gets variable x
	v, err := s.Get("x")
	assert.NoError(t, err)
	assert.Equal(t, []byte{42}, v)

	// mutation is in currentStateUpdate, prefixed by the contract id
	assert.Equal(t, []byte{42}, stateUpdate.Mutations.Sets[subpartitionedKey])

	// mutation is in the not committed to the virtual state yet
	v, err = stateDraft.Get(subpartitionedKey)
	assert.NoError(t, err)
	assert.Nil(t, v)

	// contract deletes variable x
	s.Del("x")
	assert.Equal(t, map[kv.Key][]byte{}, stateUpdate.Mutations.Sets)
	assert.Equal(t, map[kv.Key]struct{}{subpartitionedKey: {}}, stateUpdate.Mutations.Dels)

	// contract sees variable x does not exist
	v, err = s.Get("x")
	assert.NoError(t, err)
	assert.Nil(t, v)

	// contract makes several writes to same variable, gets the latest value
	s.Set("x", []byte{2 * 42})
	assert.Equal(t, map[kv.Key][]byte{subpartitionedKey: {2 * 42}}, stateUpdate.Mutations.Sets)
	assert.Equal(t, map[kv.Key]struct{}{}, stateUpdate.Mutations.Dels)

	s.Set("x", []byte{3 * 42})
	assert.Equal(t, map[kv.Key][]byte{subpartitionedKey: {3 * 42}}, stateUpdate.Mutations.Sets)
	assert.Equal(t, map[kv.Key]struct{}{}, stateUpdate.Mutations.Dels)

	v, err = s.Get("x")

	assert.NoError(t, err)
	assert.Equal(t, []byte{3 * 42}, v)
}

func TestIterate(t *testing.T) {
	db := mapdb.NewMapDB()
	cs := origin.InitChain(state.NewStore(db), nil, 0)
	latest, err := cs.LatestBlock()
	assert.NoError(t, err)
	stateDraft, err := cs.NewStateDraft(time.Now(), latest.L1Commitment())
	assert.NoError(t, err)

	stateUpdate := NewStateUpdate()
	hname := isc.Hn("test")

	vmctx := &VMContext{
		task:               &vm.VMTask{StateDraft: stateDraft},
		currentStateUpdate: stateUpdate,
		callStack:          []*callContext{{contract: hname}},
	}
	s := vmctx.State()
	s.Set("xy1", []byte{42})
	s.Set("xy2", []byte{42 * 2})

	arr := make([][]byte, 0)
	err = s.IterateSorted("xy", func(k kv.Key, v []byte) bool {
		assert.True(t, strings.HasPrefix(string(k), "xy"))
		arr = append(arr, v)
		return true
	})
	require.EqualValues(t, 2, len(arr))
	require.Equal(t, []byte{42}, arr[0])
	require.Equal(t, []byte{42 * 2}, arr[1])
	assert.NoError(t, err)
}

func TestVmctxStateDeletion(t *testing.T) {
	db := mapdb.NewMapDB()
	cs := origin.InitChain(state.NewStore(db), nil, 0)

	foo := kv.Key("foo")
	{
		latest, err := cs.LatestBlock()
		assert.NoError(t, err)
		stateDraft, err := cs.NewStateDraft(time.Now(), latest.L1Commitment())
		assert.NoError(t, err)
		stateDraft.Set(foo, []byte("bar"))
		block := cs.Commit(stateDraft)
		err = cs.SetLatest(block.TrieRoot())
		require.NoError(t, err)
	}

	latest, err := cs.LatestBlock()
	assert.NoError(t, err)
	stateDraft, err := cs.NewStateDraft(time.Now(), latest.L1Commitment())
	assert.NoError(t, err)
	stateUpdate := NewStateUpdate()
	vmctx := &VMContext{
		task:               &vm.VMTask{StateDraft: stateDraft},
		currentStateUpdate: stateUpdate,
	}
	vmctxStore := vmctx.chainState()
	require.EqualValues(t, "bar", vmctxStore.MustGet(foo))
	vmctxStore.Del(foo)
	require.False(t, vmctxStore.MustHas(foo))
	val := vmctxStore.MustGet(foo)
	require.Nil(t, val)
}
