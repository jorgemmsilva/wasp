// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package isc

import (
	"fmt"
	"io"
	"math"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/testutil/testiotago"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

type OutputInfo struct {
	ChainOutputs
	TransactionIDSpent iotago.TransactionID
}

func (o *OutputInfo) Consumed() bool {
	return o.TransactionIDSpent != iotago.EmptyTransactionID
}

func NewOutputInfo(chainOutputs ChainOutputs, transactionIDSpent iotago.TransactionID) *OutputInfo {
	return &OutputInfo{
		ChainOutputs:       chainOutputs,
		TransactionIDSpent: transactionIDSpent,
	}
}

type ChainOutputs struct {
	AnchorOutput    *iotago.AnchorOutput
	AnchorOutputID  iotago.OutputID
	accountOutput   *iotago.AccountOutput // nil if AnchorOutput.StateIndex == 0
	accountOutputID iotago.OutputID
}

func NewChainOuptuts(
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
	return NewChainOuptuts(
		&iotago.AnchorOutput{},
		testiotago.RandOutputID(),
		&iotago.AccountOutput{},
		testiotago.RandOutputID(),
	)
}

func ChainOutputsFromBytes(data []byte) (*ChainOutputs, error) {
	return rwutil.ReadFromBytes(data, new(ChainOutputs))
}

func (a *ChainOutputs) Bytes() []byte {
	return rwutil.WriteToBytes(a)
}

func (a *ChainOutputs) GetAnchorID() iotago.AnchorID {
	return util.AnchorIDFromAnchorOutput(a.AnchorOutput, a.AnchorOutputID)
}

func (a *ChainOutputs) AccountOutput() (iotago.OutputID, *iotago.AccountOutput, bool) {
	return a.accountOutputID, a.accountOutput, a.accountOutput != nil
}

func (a *ChainOutputs) MustAccountOutput() *iotago.AccountOutput {
	if a.accountOutput == nil {
		panic("expected account output != nil")
	}
	return a.accountOutput
}

func (a *ChainOutputs) MustAccountOutputID() iotago.OutputID {
	if a.accountOutput == nil {
		panic("expected account output != nil")
	}
	return a.accountOutputID
}

func (a *ChainOutputs) Equals(other *ChainOutputs) bool {
	if other == nil {
		return false
	}
	if !a.AnchorOutput.Equal(other.AnchorOutput) {
		return false
	}
	if a.AnchorOutputID != other.AnchorOutputID {
		return false
	}
	if a.accountOutput == nil {
		if other.accountOutput != nil {
			return false
		}
	} else {
		if !a.accountOutput.Equal(other.accountOutput) {
			return false
		}
		if a.accountOutputID != other.accountOutputID {
			return false
		}
	}
	return true
}

func (a *ChainOutputs) Hash() hashing.HashValue {
	return hashing.HashDataBlake2b(a.Bytes())
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
