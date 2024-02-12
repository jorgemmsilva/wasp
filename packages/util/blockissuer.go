package util

import (
	"fmt"

	iotago "github.com/iotaledger/iota.go/v4"
)

func BlockIssuerFromOutputs(outputs iotago.OutputSet) (iotago.AccountID, error) {
	var blockIssuerID iotago.AccountID
	// use whatever account output is present in unspent outputs
	for _, out := range outputs {
		if out.Type() == iotago.OutputAccount {
			// found the account to issue the block from, do not include it in the tx
			blockIssuerID = out.(*iotago.AccountOutput).AccountID
			continue
		}
	}
	if blockIssuerID == iotago.EmptyAccountID {
		return iotago.EmptyAccountID, fmt.Errorf("couldn't find an account output in unspent outputs")
	}
	return blockIssuerID, nil
}
