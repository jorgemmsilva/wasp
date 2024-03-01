package util

import (
	"context"
	"os"
	"time"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/clients/apiextensions"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/tools/wasp-cli/cli/cliclients"
	"github.com/iotaledger/wasp/tools/wasp-cli/cli/config"
	"github.com/iotaledger/wasp/tools/wasp-cli/log"
)

func WithOffLedgerRequest(chainID isc.ChainID, nodeName string, f func() (isc.OffLedgerRequest, error)) {
	req, err := f()
	log.Check(err)
	log.Printf("Posted off-ledger request (check result with: %s chain request %s)\n", os.Args[0], req.ID().String())
	if config.WaitForCompletion {
		receipt, _, err := cliclients.WaspClient(nodeName).ChainsApi.
			WaitForRequest(context.Background(), chainID.Bech32(cliclients.API().ProtocolParameters().Bech32HRP()), req.ID().String()).
			WaitForL1Confirmation(true).
			TimeoutSeconds(60).
			Execute()

		log.Check(err)
		LogReceipt(*receipt)
	}
}

func WithSCTransaction(chainID isc.ChainID, nodeName string, f func() (*iotago.Block, error), forceWait ...bool) *iotago.Block {
	block, err := f()
	log.Check(err)
	logTx(chainID, util.TxFromBlock(block))

	if config.WaitForCompletion || len(forceWait) > 0 {
		log.Printf("Waiting for tx requests to be processed...\n")
		client := cliclients.WaspClient(nodeName)
		_, err := apiextensions.APIWaitUntilAllRequestsProcessed(client, chainID, util.TxFromBlock(block), true, 1*time.Minute)
		log.Check(err)
	}

	return block
}

func logTx(chainID isc.ChainID, tx *iotago.SignedTransaction) {
	allReqs, err := isc.RequestsInTransaction(tx.Transaction)
	log.Check(err)
	txid, err := tx.ID()
	log.Check(err)
	reqs := allReqs[chainID]
	if len(reqs) == 0 {
		log.Printf("Posted on-ledger transaction %s\n", txid.ToHex())
	} else {
		plural := ""
		if len(reqs) != 1 {
			plural = "s"
		}
		log.Printf("Posted on-ledger transaction %s containing %d request%s:\n", txid.ToHex(), len(reqs), plural)
		for i, req := range reqs {
			log.Printf("  - #%d (check result with: %s chain request %s)\n", i, os.Args[0], req.ID().String())
		}
	}
}
