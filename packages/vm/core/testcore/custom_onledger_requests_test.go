package testcore

import (
	"math"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/contracts/native/inccounter"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/testutil/testmisc"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

func TestNoSenderFeature(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
	ch := env.NewChain()

	wallet, addr := env.NewKeyPairWithFunds()

	// ----------------------------------------------------------------

	// mint some NTs and withdraw them
	gasFee := iotago.BaseToken(10 * gas.LimitsDefault.MinGasPerRequest)
	withdrawAmount := iotago.BaseToken(3 * gas.LimitsDefault.MinGasPerRequest)
	err := ch.DepositAssetsToL2(isc.NewAssetsBaseTokens(withdrawAmount+gasFee), wallet)
	require.NoError(t, err)

	nft, _, err := ch.Env.MintNFTL1(wallet, addr, iotago.MetadataFeatureEntries{"": []byte("foobar")})
	require.NoError(t, err)

	// ----------------------------------------------------------------

	payoutAgentIDBalanceBefore := ch.L2Assets(ch.OriginatorAgentID)

	// send a custom request with Base tokens / NTs / NFT (but no sender feature)
	allOuts := ch.Env.GetUnspentOutputs(addr)
	tx, err := transaction.NewRequestTransaction(
		wallet,
		addr,
		allOuts,
		&isc.RequestParameters{
			TargetAddress: ch.ChainID.AsAddress(),
			Assets:        isc.NewAssetsBaseTokens(5 * isc.Million).AddNFTs(nft.ID),
			Metadata: &isc.SendMetadata{
				Message:   inccounter.FuncIncCounter.Message(nil),
				GasBudget: math.MaxUint64,
			},
		},
		nft,
		env.SlotIndex(),
		false,
		testutil.L1APIProvider,
	)
	require.NoError(t, err)

	// tweak the tx before adding to the ledger, so the request output has no sender feature
	for i, out := range tx.Transaction.Outputs {
		if out.FeatureSet().Metadata() == nil {
			// skip if not the request output
			continue
		}
		customOut := out.Clone().(*iotago.NFTOutput) // must be NFT output because we're sending an NFT
		customOut.Features = lo.Filter(out.(*iotago.NFTOutput).Features, func(f iotago.NFTOutputFeature, _ int) bool {
			_, ok := f.(*iotago.SenderFeature)
			return !ok
		})
		tx.Transaction.Outputs[i] = customOut
	}

	tx, err = transaction.CreateAndSignTx(
		wallet,
		tx.Transaction.TransactionEssence.Inputs,
		tx.Transaction.Outputs,
		env.SlotIndex(),
		testutil.L1API,
	)
	require.NoError(t, err)
	err = ch.Env.AddToLedger(tx)
	require.NoError(t, err)

	reqs, err := ch.Env.RequestsForChain(tx.Transaction, ch.ChainID)
	require.NoError(ch.Env.T, err)
	results := ch.RunRequestsSync(reqs, "post") // under normal circumstances this request won't reach the mempool
	require.Len(t, results, 1)
	require.NotNil(t, results[0].Receipt.Error)
	err = ch.ResolveVMError(results[0].Receipt.Error)
	testmisc.RequireErrorToBe(t, err, "sender unknown")

	// assert the assets were credited to the payout address
	payoutAgentIDBalance := ch.L2Assets(ch.OriginatorAgentID)
	require.Greater(t, payoutAgentIDBalance.BaseTokens, payoutAgentIDBalanceBefore.BaseTokens)
	require.EqualValues(t, payoutAgentIDBalance.NFTs[0], nft.ID)
}

func TestSendBack(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true}).
		WithNativeContract(inccounter.Processor)
	ch := env.NewChain()

	err := ch.DepositBaseTokensToL2(10*isc.Million, nil)
	require.NoError(t, err)

	err = ch.DeployContract(nil, inccounter.Contract.Name, inccounter.Contract.ProgramHash, inccounter.InitParams(0))
	require.NoError(t, err)

	// send a normal request
	wallet, addr := env.NewKeyPairWithFunds()

	req := solo.NewCallParams(inccounter.FuncIncCounter.Message(nil)).WithMaxAffordableGasBudget()
	_, _, err = ch.PostRequestSyncTx(req, wallet)
	require.NoError(t, err)

	// check counter increments
	ret, err := ch.CallView(inccounter.ViewGetCounter.Message())
	require.NoError(t, err)
	counter, err := inccounter.ViewGetCounter.Output.Decode(ret)
	require.NoError(t, err)
	require.EqualValues(t, 1, counter)

	// send a custom request
	allOuts := ch.Env.GetUnspentOutputs(addr)
	tx, err := transaction.NewRequestTransaction(
		wallet,
		addr,
		allOuts,
		&isc.RequestParameters{
			TargetAddress: ch.ChainID.AsAddress(),
			Assets:        isc.NewAssetsBaseTokens(1 * isc.Million),
			Metadata: &isc.SendMetadata{
				Message:   inccounter.FuncIncCounter.Message(nil),
				GasBudget: math.MaxUint64,
			},
		},
		nil,
		env.SlotIndex(),
		false,
		testutil.L1APIProvider,
	)
	require.NoError(t, err)

	// tweak the tx before adding to the ledger, so the request output has a StorageDepositReturn unlock condition
	for i, out := range tx.Transaction.Outputs {
		if out.FeatureSet().Metadata() == nil {
			// skip if not the request output
			continue
		}
		customOut := out.Clone().(*iotago.BasicOutput)
		sendBackCondition := &iotago.StorageDepositReturnUnlockCondition{
			ReturnAddress: addr,
			Amount:        1 * isc.Million,
		}
		customOut.UnlockConditions.Upsert(sendBackCondition)
		customOut.UnlockConditions.Sort()
		tx.Transaction.Outputs[i] = customOut
	}

	tx, err = transaction.CreateAndSignTx(
		wallet,
		tx.Transaction.TransactionEssence.Inputs,
		tx.Transaction.Outputs,
		env.SlotIndex(),
		testutil.L1API,
	)
	require.NoError(t, err)
	err = ch.Env.AddToLedger(tx)
	require.NoError(t, err)

	reqs, err := ch.Env.RequestsForChain(tx.Transaction, ch.ChainID)
	require.NoError(ch.Env.T, err)
	results := ch.RunRequestsSync(reqs, "post")
	// TODO for now the request must be skipped, in the future this needs to be refactored, so that the request is handled as expected
	require.Len(t, results, 0)

	// check counter is still the same (1)
	ret, err = ch.CallView(inccounter.ViewGetCounter.Message())
	require.NoError(t, err)
	counter, err = inccounter.ViewGetCounter.Output.Decode(ret)
	require.NoError(t, err)
	require.EqualValues(t, 1, counter)
}

func TestBadMetadata(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
	ch := env.NewChain()

	wallet, addr := env.NewKeyPairWithFunds()

	// send a custom request
	allOuts := ch.Env.GetUnspentOutputs(addr)
	tx, err := transaction.NewRequestTransaction(
		wallet,
		addr,
		allOuts,
		&isc.RequestParameters{
			TargetAddress: ch.ChainID.AsAddress(),
			Assets:        isc.NewAssetsBaseTokens(1 * isc.Million),
			Metadata: &isc.SendMetadata{
				Message:   inccounter.FuncIncCounter.Message(nil),
				GasBudget: math.MaxUint64,
			},
		},
		nil,
		env.SlotIndex(),
		false,
		testutil.L1APIProvider,
	)
	require.NoError(t, err)

	// tweak the tx before adding to the ledger, set bad metadata
	for i, out := range tx.Transaction.Outputs {
		if out.FeatureSet().Metadata() == nil {
			// skip if not the request output
			continue
		}
		customOut := out.Clone().(*iotago.BasicOutput)
		for ii, f := range customOut.Features {
			if mf, ok := f.(*iotago.MetadataFeature); ok {
				mf.Entries = iotago.MetadataFeatureEntries{"": []byte("foobar")}
				customOut.Features[ii] = mf
			}
		}
		tx.Transaction.Outputs[i] = customOut
	}

	tx, err = transaction.CreateAndSignTx(
		wallet,
		tx.Transaction.TransactionEssence.Inputs,
		tx.Transaction.Outputs,
		env.SlotIndex(),
		testutil.L1API,
	)
	require.NoError(t, err)
	require.Zero(t, ch.L2BaseTokens(isc.NewAddressAgentID(addr)))
	err = ch.Env.AddToLedger(tx)
	require.NoError(t, err)

	reqs, err := ch.Env.RequestsForChain(tx.Transaction, ch.ChainID)
	require.NoError(ch.Env.T, err)
	results := ch.RunRequestsSync(reqs, "post")
	// assert request was processed with an error
	require.Len(t, results, 1)
	require.NotNil(t, results[0].Receipt.Error)
	err = ch.ResolveVMError(results[0].Receipt.Error)
	testmisc.RequireErrorToBe(t, err, "contract with hname 3030303030303030 not found")

	// assert funds were credited to the sender
	require.Positive(t, ch.L2BaseTokens(isc.NewAddressAgentID(addr)))
}
