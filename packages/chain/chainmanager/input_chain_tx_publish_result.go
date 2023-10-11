package chainmanager

import (
	"fmt"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/chain/cmt_log"
	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
)

type inputChainTxPublishResult struct {
	committeeAddr iotago.Ed25519Address
	logIndex      cmt_log.LogIndex
	txID          iotago.TransactionID
	accountOutput   *isc.AccountOutputWithID
	confirmed     bool
}

func NewInputChainTxPublishResult(committeeAddr iotago.Ed25519Address, logIndex cmt_log.LogIndex, txID iotago.TransactionID, accountOutput *isc.AccountOutputWithID, confirmed bool) gpa.Input {
	return &inputChainTxPublishResult{
		committeeAddr: committeeAddr,
		logIndex:      logIndex,
		txID:          txID,
		accountOutput:   accountOutput,
		confirmed:     confirmed,
	}
}

func (i *inputChainTxPublishResult) String() string {
	return fmt.Sprintf(
		"{chainMgr.inputChainTxPublishResult, committeeAddr=%v, logIndex=%v, txID=%v, accountOutput=%v, confirmed=%v}",
		i.committeeAddr.String(),
		i.logIndex,
		i.txID.ToHex(),
		i.accountOutput,
		i.confirmed,
	)
}
