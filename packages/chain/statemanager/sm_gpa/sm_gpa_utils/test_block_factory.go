// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package sm_gpa_utils

import (
	"crypto/rand"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/origin"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/testutil"
)

type BlockFactory struct {
	t                   require.TestingT
	store               state.Store
	chainID             isc.ChainID
	chainInitParams     dict.Dict
	lastBlockCommitment *state.L1Commitment
	anchorOutputs       map[state.BlockHash]*isc.ChainOutputs
}

func NewBlockFactory(t require.TestingT, chainInitParamsOpt ...dict.Dict) *BlockFactory {
	var chainInitParams dict.Dict
	if len(chainInitParamsOpt) > 0 {
		chainInitParams = chainInitParamsOpt[0]
	} else {
		chainInitParams = nil
	}
	anchorOutput0ID := iotago.OutputIDFromTransactionIDAndIndex(getRandomTxID(t), 0)
	chainID := isc.ChainIDFromAnchorID(iotago.AnchorIDFromOutputID(anchorOutput0ID))
	stateAddress := cryptolib.NewKeyPair().GetPublicKey().AsEd25519Address()
	originCommitment := origin.L1Commitment(0, chainInitParams, 0, testutil.TokenInfo)
	anchorOutput0 := &iotago.AnchorOutput{
		Amount:   testutil.L1API.ProtocolParameters().TokenSupply(),
		AnchorID: chainID.AsAnchorID(), // NOTE: not very correct: origin output's AccountID should be empty; left here to make mocking transitions easier
		UnlockConditions: iotago.AnchorOutputUnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{Address: stateAddress},
			&iotago.GovernorAddressUnlockCondition{Address: stateAddress},
		},
		Features: iotago.AnchorOutputFeatures{
			&iotago.StateMetadataFeature{
				Entries: map[iotago.StateMetadataFeatureEntriesKey]iotago.StateMetadataFeatureEntriesValue{
					"": testutil.DummyStateMetadata(originCommitment).Bytes(),
				},
			},
			&iotago.SenderFeature{
				Address: stateAddress,
			},
		},
	}
	anchorOutputs := make(map[state.BlockHash]*isc.ChainOutputs)
	originOutput := isc.NewChainOutputs(anchorOutput0, anchorOutput0ID, nil, iotago.OutputID{})
	anchorOutputs[originCommitment.BlockHash()] = originOutput
	chainStore := state.NewStoreWithUniqueWriteMutex(mapdb.NewMapDB())
	origin.InitChain(0, chainStore, chainInitParams, 0, testutil.TokenInfo)
	return &BlockFactory{
		t:                   t,
		store:               chainStore,
		chainID:             chainID,
		chainInitParams:     chainInitParams,
		lastBlockCommitment: originCommitment,
		anchorOutputs:       anchorOutputs,
	}
}

func (bfT *BlockFactory) GetChainID() isc.ChainID {
	return bfT.chainID
}

func (bfT *BlockFactory) GetChainInitParameters() dict.Dict {
	return bfT.chainInitParams
}

func (bfT *BlockFactory) GetOriginOutput() *isc.ChainOutputs {
	return bfT.GetChainOutputs(origin.L1Commitment(0, bfT.chainInitParams, 0, testutil.TokenInfo))
}

func (bfT *BlockFactory) GetOriginBlock() state.Block {
	block, err := bfT.store.BlockByTrieRoot(origin.L1Commitment(0, bfT.chainInitParams, 0, testutil.TokenInfo).TrieRoot())
	require.NoError(bfT.t, err)
	return block
}

func (bfT *BlockFactory) GetBlocks(
	count,
	branchingFactor int,
) []state.Block {
	blocks := bfT.GetBlocksFrom(count, branchingFactor, bfT.lastBlockCommitment)
	require.Equal(bfT.t, count, len(blocks))
	bfT.lastBlockCommitment = blocks[count-1].L1Commitment()
	return blocks
}

func (bfT *BlockFactory) GetBlocksFrom(
	count,
	branchingFactor int,
	commitment *state.L1Commitment,
	incrementFactorOpt ...uint64,
) []state.Block {
	var incrementFactor uint64
	if len(incrementFactorOpt) > 0 {
		incrementFactor = incrementFactorOpt[0]
	} else {
		incrementFactor = 1
	}
	result := make([]state.Block, count+1)
	var err error
	result[0], err = bfT.store.BlockByTrieRoot(commitment.TrieRoot())
	require.NoError(bfT.t, err)
	for i := 1; i < len(result); i++ {
		baseIndex := (i + branchingFactor - 2) / branchingFactor
		increment := uint64(1+i%branchingFactor) * incrementFactor
		result[i] = bfT.GetNextBlock(result[baseIndex].L1Commitment(), increment)
	}
	return result[1:]
}

func (bfT *BlockFactory) GetNextBlock(
	commitment *state.L1Commitment,
	incrementOpt ...uint64,
) state.Block {
	stateDraft, err := bfT.store.NewStateDraft(time.Now(), commitment)
	require.NoError(bfT.t, err)
	counterKey := kv.Key(coreutil.StateVarBlockIndex + "counter")
	counterBin := stateDraft.Get(counterKey)
	counter, err := codec.Uint64.Decode(counterBin, 0)
	require.NoError(bfT.t, err)
	var increment uint64
	if len(incrementOpt) > 0 {
		increment = incrementOpt[0]
	} else {
		increment = 1
	}
	counterBin = codec.Uint64.Encode(counter + increment)
	stateDraft.Mutations().Set(counterKey, counterBin)
	block := bfT.store.Commit(stateDraft)
	// require.EqualValues(t, stateDraft.BlockIndex(), block.BlockIndex())
	newCommitment := block.L1Commitment()

	consumedAnchorOutput := bfT.GetChainOutputs(commitment).AnchorOutput

	newFeatures := lo.Map(consumedAnchorOutput.Features, func(f iotago.Feature, i int) iotago.Feature {
		_, ok := f.(*iotago.StateMetadataFeature)
		if ok {
			return &iotago.StateMetadataFeature{
				Entries: map[iotago.StateMetadataFeatureEntriesKey]iotago.StateMetadataFeatureEntriesValue{
					"": testutil.DummyStateMetadata(newCommitment).Bytes(),
				},
			}
		}
		return f
	})

	anchorOutput := &iotago.AnchorOutput{
		Amount:            consumedAnchorOutput.Amount,
		Mana:              consumedAnchorOutput.Mana,
		AnchorID:          consumedAnchorOutput.AnchorID,
		StateIndex:        consumedAnchorOutput.StateIndex + 1,
		UnlockConditions:  consumedAnchorOutput.UnlockConditions,
		Features:          newFeatures,
		ImmutableFeatures: consumedAnchorOutput.ImmutableFeatures,
	}
	anchorOutputID := iotago.OutputIDFromTransactionIDAndIndex(getRandomTxID(bfT.t), 0)
	anchorOutputWithID := isc.NewChainOutputs(anchorOutput, anchorOutputID, nil, iotago.OutputID{})
	bfT.anchorOutputs[newCommitment.BlockHash()] = anchorOutputWithID

	return block
}

func (bfT *BlockFactory) GetStore() state.Store {
	return NewReadOnlyStore(bfT.store)
}

func (bfT *BlockFactory) GetStateDraft(block state.Block) state.StateDraft {
	result, err := bfT.store.NewEmptyStateDraft(block.PreviousL1Commitment())
	require.NoError(bfT.t, err)
	block.Mutations().ApplyTo(result)
	return result
}

func (bfT *BlockFactory) GetChainOutputs(commitment *state.L1Commitment) *isc.ChainOutputs {
	result, ok := bfT.anchorOutputs[commitment.BlockHash()]
	require.True(bfT.t, ok)
	return result
}

func getRandomTxID(t require.TestingT) iotago.TransactionID {
	var result iotago.TransactionID
	_, err := rand.Read(result[:])
	require.NoError(t, err)
	return result
}
