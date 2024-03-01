package isc

import (
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/core/types"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

// Request wraps any data which can be potentially be interpreted as a request
type Request interface {
	Calldata

	Bytes() []byte
	IsOffLedger() bool
	String(iotago.NetworkPrefix) string

	Read(r io.Reader) error
	Write(w io.Writer) error
}

type Calldata interface {
	Allowance() *Assets // transfer of assets to the smart contract. Debited from sender anchor
	Assets() *Assets    // attached assets for the UTXO request, nil for off-ledger. All goes to sender
	Message() Message
	GasBudget() (gas uint64, isEVM bool)
	ID() RequestID
	NFT() *NFT // Not nil if the request is an NFT request
	SenderAccount() AgentID
	TargetAddress() iotago.Address
}

type Features interface {
	Expiry() (iotago.SlotIndex, iotago.Address, bool)
	ReturnAmount() (iotago.BaseToken, bool)
	TimeLock() (iotago.SlotIndex, bool)
}

type UnsignedOffLedgerRequest interface {
	Bytes() []byte
	WithNonce(nonce uint64) UnsignedOffLedgerRequest
	WithGasBudget(gasBudget gas.GasUnits) UnsignedOffLedgerRequest
	WithAllowance(allowance *Assets) UnsignedOffLedgerRequest
	WithSender(sender *cryptolib.PublicKey) UnsignedOffLedgerRequest
	Sign(key cryptolib.VariantKeyPair) OffLedgerRequest
}

type OffLedgerRequest interface {
	Request
	ChainID() ChainID
	Nonce() uint64
	VerifySignature() error
	EVMTransaction() *types.Transaction
}

type OnLedgerRequest interface {
	Request
	Clone() OnLedgerRequest
	Output() iotago.Output
	IsInternalUTXO(ChainID) bool
	OutputID() iotago.OutputID
	Features() Features
}

type ReturnAmountOptions interface {
	ReturnTo() iotago.Address
	Amount() uint64
}

// RequestsInTransaction parses the transaction and extracts those outputs which are interpreted as a request to a chain
func RequestsInTransaction(tx *iotago.Transaction) (map[ChainID][]Request, error) {
	txid, err := tx.ID()
	if err != nil {
		return nil, err
	}
	if tx.TransactionEssence == nil {
		return nil, fmt.Errorf("malformed transaction")
	}

	ret := make(map[ChainID][]Request)
	for i, output := range tx.Outputs {
		switch output.(type) {
		case *iotago.BasicOutput, *iotago.NFTOutput:
			// process it
		default:
			// only BasicOutputs and NFTs are interpreted right now, // TODO other outputs
			continue
		}

		// wrap output into the isc.Request
		odata, err := OnLedgerFromUTXO(output, iotago.OutputIDFromTransactionIDAndIndex(txid, uint16(i)))
		if err != nil {
			return nil, err // TODO: maybe log the error and keep processing?
		}

		addr := odata.TargetAddress()
		if addr.Type() != iotago.AddressAnchor {
			continue
		}

		chainID := ChainIDFromAnchorID(addr.(*iotago.AnchorAddress).AnchorID())

		if odata.IsInternalUTXO(chainID) {
			continue
		}

		ret[chainID] = append(ret[chainID], odata)
	}
	return ret, nil
}

func RequestIsExpired(req OnLedgerRequest, currentSlotIndex iotago.SlotIndex) bool {
	expiry, _, ok := req.Features().Expiry()
	if !ok {
		return false
	}
	return currentSlotIndex >= expiry
}

func RequestIsUnlockable(req OnLedgerRequest, chainAddress iotago.Address, pastBoundedSlotIndex, futureBoundedSlotIndex iotago.SlotIndex) bool {
	output, _ := req.Output().(iotago.OwnerTransitionIndependentOutput)
	return output.UnlockableBy(chainAddress, pastBoundedSlotIndex, futureBoundedSlotIndex)
}

func RequestHash(req Request) hashing.HashValue {
	return hashing.HashData(req.Bytes())
}
