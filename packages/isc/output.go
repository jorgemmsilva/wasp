// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package isc

import (
	"bytes"
	"fmt"
	"io"
	"math"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/testutil/testiotago"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

var emptyTransactionID = iotago.TransactionID{}

type OutputInfo struct {
	OutputID           iotago.OutputID
	Output             iotago.Output
	TransactionIDSpent iotago.TransactionID
}

func (o *OutputInfo) Consumed() bool {
	return o.TransactionIDSpent != emptyTransactionID
}

func NewOutputInfo(outputID iotago.OutputID, output iotago.Output, transactionIDSpent iotago.TransactionID) *OutputInfo {
	return &OutputInfo{
		OutputID:           outputID,
		Output:             output,
		TransactionIDSpent: transactionIDSpent,
	}
}

func (o *OutputInfo) AccountOutputWithID() *AccountOutputWithID {
	return NewAccountOutputWithID(o.Output.(*iotago.AccountOutput), o.OutputID)
}

type AccountOutputWithID struct {
	outputID      iotago.OutputID
	accountOutput *iotago.AccountOutput
}

func NewAccountOutputWithID(accountOutput *iotago.AccountOutput, outputID iotago.OutputID) *AccountOutputWithID {
	return &AccountOutputWithID{
		outputID:      outputID,
		accountOutput: accountOutput,
	}
}

// only for testing
func RandomAccountOutputWithID() *AccountOutputWithID {
	outputID := testiotago.RandOutputID()
	accountOutput := &iotago.AccountOutput{}
	return NewAccountOutputWithID(accountOutput, outputID)
}

func AccountOutputWithIDFromBytes(data []byte) (*AccountOutputWithID, error) {
	return rwutil.ReadFromBytes(data, new(AccountOutputWithID))
}

func (a *AccountOutputWithID) Bytes() []byte {
	return rwutil.WriteToBytes(a)
}

func (a *AccountOutputWithID) GetAccountOutput() *iotago.AccountOutput {
	return a.accountOutput
}

func (a *AccountOutputWithID) OutputID() iotago.OutputID {
	return a.outputID
}

func (a *AccountOutputWithID) TransactionID() iotago.TransactionID {
	return a.outputID.TransactionID()
}

func (a *AccountOutputWithID) GetStateIndex() uint32 {
	return a.accountOutput.StateIndex
}

func (a *AccountOutputWithID) GetStateMetadata() []byte {
	return a.accountOutput.StateMetadata
}

func (a *AccountOutputWithID) GetStateAddress() iotago.Address {
	return a.accountOutput.StateController()
}

func (a *AccountOutputWithID) GetAccountID() iotago.AccountID {
	return util.AccountIDFromAccountOutput(a.accountOutput, a.outputID)
}

func (a *AccountOutputWithID) Equals(other *AccountOutputWithID) bool {
	if other == nil {
		return false
	}
	if a.outputID != other.outputID {
		return false
	}
	ww1 := rwutil.NewBytesWriter()
	ww1.WriteSerialized(a.accountOutput, math.MaxInt32)
	ww2 := rwutil.NewBytesWriter()
	ww2.WriteSerialized(other.accountOutput, math.MaxInt32)
	return bytes.Equal(ww1.Bytes(), ww2.Bytes())
}

func (a *AccountOutputWithID) Hash() hashing.HashValue {
	return hashing.HashDataBlake2b(a.Bytes())
}

func (a *AccountOutputWithID) String() string {
	if a == nil {
		return "nil"
	}
	return fmt.Sprintf("AO[si#%v]%v", a.GetStateIndex(), a.outputID.ToHex())
}

func (a *AccountOutputWithID) Read(r io.Reader) error {
	rr := rwutil.NewReader(r)
	rr.ReadN(a.outputID[:])
	a.accountOutput = new(iotago.AccountOutput)
	rr.ReadSerialized(a.accountOutput, math.MaxInt32)
	return rr.Err
}

func (a *AccountOutputWithID) Write(w io.Writer) error {
	ww := rwutil.NewWriter(w)
	ww.WriteN(a.outputID[:])
	ww.WriteSerialized(a.accountOutput, math.MaxInt32)
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

func AccountOutputWithIDFromTx(tx *iotago.Transaction, aliasAddr iotago.Address) (*AccountOutputWithID, error) {
	txID, err := tx.ID()
	if err != nil {
		return nil, err
	}

	for index, output := range tx.Outputs {
		if accountOutput, ok := output.(*iotago.AccountOutput); ok {
			outputID := iotago.OutputIDFromTransactionIDAndIndex(txID, uint16(index))

			aliasID := accountOutput.AccountID
			if aliasID.Empty() {
				aliasID = iotago.AccountIDFromOutputID(outputID)
			}

			if aliasID.ToAddress().Equal(aliasAddr) {
				// output found
				return NewAccountOutputWithID(accountOutput, outputID), nil
			}
		}
	}

	return nil, fmt.Errorf("cannot find alias output for address %v in transaction", aliasAddr.String())
}
