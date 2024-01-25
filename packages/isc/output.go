// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package isc

import (
	"fmt"
	"io"
	"math"

	"github.com/samber/lo"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/testutil/testiotago"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

type OutputInfo struct {
	OutputID           iotago.OutputID
	Output             iotago.Output
	TransactionIDSpent iotago.TransactionID
}

func (o *OutputInfo) Consumed() bool {
	return o.TransactionIDSpent != iotago.EmptyTransactionID
}

func NewOutputInfo(outputID iotago.OutputID, output iotago.Output, transactionIDSpent iotago.TransactionID) *OutputInfo {
	return &OutputInfo{
		OutputID:           outputID,
		Output:             output,
		TransactionIDSpent: transactionIDSpent,
	}
}

type ChainOutputs struct {
	AnchorOutput    *iotago.AnchorOutput
	AnchorOutputID  iotago.OutputID
	accountOutput   *iotago.AccountOutput // nil if AnchorOutput.StateIndex == 0
	accountOutputID iotago.OutputID
}

func NewChainOutputs(
	AnchorOutput *iotago.AnchorOutput,
	anchorOutputID iotago.OutputID,
	accountOutput *iotago.AccountOutput,
	accountOutputID iotago.OutputID,
) *ChainOutputs {
	return &ChainOutputs{
		AnchorOutput:    AnchorOutput,
		AnchorOutputID:  anchorOutputID,
		accountOutput:   accountOutput,
		accountOutputID: accountOutputID,
	}
}

// only for testing
func RandomChainOutputs() *ChainOutputs {
	return NewChainOutputs(
		&iotago.AnchorOutput{
			Features: iotago.AnchorOutputFeatures{
				&iotago.StateMetadataFeature{Entries: iotago.StateMetadataFeatureEntries{}},
			},
		},
		testiotago.RandOutputID(),
		&iotago.AccountOutput{},
		testiotago.RandOutputID(),
	)
}

func ChainOutputsFromBytes(data []byte) (*ChainOutputs, error) {
	return rwutil.ReadFromBytes(data, new(ChainOutputs))
}

func (c *ChainOutputs) Bytes() []byte {
	return rwutil.WriteToBytes(c)
}

func (c *ChainOutputs) GetStateIndex() uint32 {
	return c.AnchorOutput.StateIndex
}

func (c *ChainOutputs) GetAnchorID() iotago.AnchorID {
	return util.AnchorIDFromAnchorOutput(c.AnchorOutput, c.AnchorOutputID)
}

func (c *ChainOutputs) HasAccountOutput() bool {
	return c.accountOutput != nil
}

func (c *ChainOutputs) AccountOutput() (iotago.OutputID, *iotago.AccountOutput, bool) {
	return c.accountOutputID, c.accountOutput, c.accountOutput != nil
}

func (c *ChainOutputs) Map(f func(iotago.TxEssenceOutput)) {
	f(c.AnchorOutput)
	if c.accountOutput != nil {
		f(c.accountOutput)
	}
}

func (c *ChainOutputs) MustAccountOutput() *iotago.AccountOutput {
	if c.accountOutput == nil {
		panic("expected account output != nil")
	}
	return c.accountOutput
}

func (c *ChainOutputs) MustAccountOutputID() iotago.OutputID {
	if c.accountOutput == nil {
		panic("expected account output != nil")
	}
	return c.accountOutputID
}

func (c *ChainOutputs) Equals(other *ChainOutputs) bool {
	if other == nil {
		return false
	}
	if !c.AnchorOutput.Equal(other.AnchorOutput) {
		return false
	}
	if c.AnchorOutputID != other.AnchorOutputID {
		return false
	}
	if c.accountOutput == nil {
		if other.accountOutput != nil {
			return false
		}
	} else {
		if !c.accountOutput.Equal(other.accountOutput) {
			return false
		}
		if c.accountOutputID != other.accountOutputID {
			return false
		}
	}
	return true
}

func (c *ChainOutputs) Hash() hashing.HashValue {
	return hashing.HashDataBlake2b(c.Bytes())
}

func (c *ChainOutputs) L1API(l1 iotago.APIProvider) iotago.API {
	return l1.APIForSlot(c.AnchorOutputID.CreationSlot())
}

func (c *ChainOutputs) StorageDeposit(l1 iotago.APIProvider) iotago.BaseToken {
	api := c.L1API(l1)
	sd := lo.Must(api.StorageScoreStructure().MinDeposit(c.AnchorOutput))
	if c.accountOutput != nil {
		sd += lo.Must(api.StorageScoreStructure().MinDeposit(c.accountOutput))
	}
	return sd
}

func (a *ChainOutputs) String() string {
	if a == nil {
		return "nil"
	}
	return fmt.Sprintf("AO[si#%v]%v", a.AnchorOutput.StateIndex, a.AnchorOutputID.ToHex())
}

func (a *ChainOutputs) Read(r io.Reader) error {
	rr := rwutil.NewReader(r)
	rr.ReadN(a.AnchorOutputID[:])
	a.AnchorOutput = new(iotago.AnchorOutput)
	rr.ReadSerialized(a.AnchorOutput, math.MaxInt32)
	if a.AnchorOutput.StateIndex >= 1 {
		rr.ReadN(a.accountOutputID[:])
		a.accountOutput = new(iotago.AccountOutput)
		rr.ReadSerialized(a.accountOutput, math.MaxInt32)
	}
	return rr.Err
}

func (a *ChainOutputs) Write(w io.Writer) error {
	ww := rwutil.NewWriter(w)
	ww.WriteN(a.AnchorOutputID[:])
	ww.WriteSerialized(a.AnchorOutput, math.MaxInt32)
	if a.AnchorOutput.StateIndex >= 1 {
		ww.WriteN(a.accountOutputID[:])
		ww.WriteSerialized(a.accountOutput, math.MaxInt32)
	}
	return ww.Err
}

func OutputSetToOutputIDs(outputSet iotago.OutputSet) iotago.OutputIDs {
	outputIDs := make(iotago.OutputIDs, len(outputSet))
	i := 0
	for id := range outputSet {
		outputIDs[i] = id
		i++
	}
	return outputIDs
}

func ChainOutputsFromTx(tx *iotago.Transaction, anchorAddr iotago.Address) (*ChainOutputs, error) {
	txID, err := tx.ID()
	if err != nil {
		return nil, err
	}

	ret := &ChainOutputs{}
	for index, output := range tx.Outputs {
		if anchorOutput, ok := output.(*iotago.AnchorOutput); ok {
			anchorOutputID := iotago.OutputIDFromTransactionIDAndIndex(txID, uint16(index))
			anchorID := anchorOutput.AnchorID
			if anchorID.Empty() {
				anchorID = iotago.AnchorIDFromOutputID(anchorOutputID)
			}
			if anchorID.ToAddress().Equal(anchorAddr) {
				if ret.AnchorOutput != nil {
					panic("inconsistency: multiple anchor outputs for chain in tx")
				}
				ret.AnchorOutput = anchorOutput
				ret.AnchorOutputID = anchorOutputID
			}
		}
		if accountOutput, ok := output.(*iotago.AccountOutput); ok {
			if addr := accountOutput.UnlockConditionSet().Address(); addr != nil && addr.Address.Equal(anchorAddr) {
				if ret.accountOutput != nil {
					panic("inconsistency: multiple account outputs for chain in tx")
				}
				ret.accountOutput = accountOutput
				ret.accountOutputID = iotago.OutputIDFromTransactionIDAndIndex(txID, uint16(index))
			}
		}
	}

	if ret.AnchorOutput == nil {
		return nil, fmt.Errorf("cannot find anchor output for address %v in transaction", anchorAddr.String())
	}
	return ret, nil
}
