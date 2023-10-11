package isc

import (
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/core/types"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/kv/dict"
)

// Request wraps any data which can be potentially be interpreted as a request
type Request interface {
	Calldata

	Bytes() []byte
	IsOffLedger() bool
	String() string

	Read(r io.Reader) error
	Write(w io.Writer) error
}

type Calldata interface {
	Allowance() *Assets // transfer of assets to the smart contract. Debited from sender account
	Assets() *Assets    // attached assets for the UTXO request, nil for off-ledger. All goes to sender
	CallTarget() CallTarget
	GasBudget() (gas uint64, isEVM bool)
	ID() RequestID
	NFT() *NFT // Not nil if the request is an NFT request
	Params() dict.Dict
	SenderAccount() AgentID
	TargetAddress() iotago.Address // TODO implement properly. Target depends on time assumptions and UTXO type
}

type Features interface {
	Expiry() (iotago.SlotIndex, iotago.Address, bool)
	ReturnAmount() (iotago.BaseToken, bool)
	TimeLock() (iotago.SlotIndex, bool)
}

type UnsignedOffLedgerRequest interface {
	Bytes() []byte
	WithNonce(nonce uint64) UnsignedOffLedgerRequest
	WithGasBudget(gasBudget uint64) UnsignedOffLedgerRequest
	WithAllowance(allowance *Assets) UnsignedOffLedgerRequest
	WithSender(sender *cryptolib.PublicKey) UnsignedOffLedgerRequest
	Sign(key *cryptolib.KeyPair) OffLedgerRequest
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

func MustLogRequestsInTransaction(tx *iotago.Transaction, log func(msg string, args ...interface{}), prefix string) {
	txReqs, err := RequestsInTransaction(tx)
	if err != nil {
		panic(fmt.Errorf("cannot extract requests from TX: %w", err))
	}
	for chainID, chainReqs := range txReqs {
		for i, req := range chainReqs {
			log("%v, ChainID=%v, Req[%v]=%v", prefix, chainID.ShortString(), i, req.String())
		}
	}
}

// RequestsInTransaction parses the transaction and extracts those outputs which are interpreted as a request to a chain
func RequestsInTransaction(tx *iotago.Transaction) (map[ChainID][]Request, error) {
	txid, err := tx.ID()
	if err != nil {
		return nil, err
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
		if addr.Type() != iotago.AddressAccount {
			continue
		}

		chainID := ChainIDFromAccountID(addr.(*iotago.AccountAddress).AccountID())

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
	output, _ := req.Output().(iotago.TransIndepIdentOutput)
	return output.UnlockableBy(chainAddress, pastBoundedSlotIndex, futureBoundedSlotIndex)
}

func RequestHash(req Request) hashing.HashValue {
	return hashing.HashData(req.Bytes())
}
