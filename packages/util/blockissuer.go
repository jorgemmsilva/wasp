package util

import (
	"fmt"

	iotago "github.com/iotaledger/iota.go/v4"
)

func BlockIssuerAccountIDFromOutputs(outputs iotago.OutputSet) (iotago.AccountID, error) {
	var blockIssuerID iotago.AccountID
	// use whatever account output is present in unspent outputs
	for outID, out := range outputs {
		if out.Type() == iotago.OutputAccount {
			// found the account to issue the block from, do not include it in the tx
			blockIssuerID = out.(*iotago.AccountOutput).AccountID
			if blockIssuerID == iotago.EmptyAccountID {
				return iotago.AccountIDFromOutputID(outID), nil
			}
			return blockIssuerID, nil
		}
	}
	return iotago.EmptyAccountID, fmt.Errorf("couldn't find an account output in unspent outputs")
}

func AccountOutputFromOutputs(outputs iotago.OutputSet) (iotago.OutputID, *iotago.AccountOutput) {
	// use whatever account output is present in unspent outputs
	for id, out := range outputs {
		if out.Type() == iotago.OutputAccount {
			// found the account to issue the block from, do not include it in the tx
			return id, out.(*iotago.AccountOutput)
		}
	}
	return iotago.EmptyOutputID, nil
}

func TxFromBlock(block *iotago.Block) *iotago.SignedTransaction {
	return block.Body.(*iotago.BasicBlockBody).Payload.(*iotago.SignedTransaction)
}

func AccountIDFromOutputAndID(o *iotago.AccountOutput, id iotago.OutputID) iotago.AccountID {
	if o.AccountID == iotago.EmptyAccountID {
		return iotago.AccountIDFromOutputID(id)
	}
	return o.AccountID
}
