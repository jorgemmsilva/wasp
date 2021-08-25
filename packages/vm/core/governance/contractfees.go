package governance

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/wasp/packages/iscp/colored"
)

// ContractFeesRecord is a structure which contains the fee information for a contract
type ContractFeesRecord struct {
	// Chain owner part of the fee. If it is 0, it means chain-global default is in effect
	OwnerFee uint64
	// Validator part of the fee. If it is 0, it means chain-global default is in effect
	ValidatorFee uint64
	// Color of the fee
	FeeColor colored.Color
}

func NewContractFeesRecord(ownerFee, validatorFee uint64, color ...colored.Color) *ContractFeesRecord {
	feeColor := colored.IOTA
	if len(color) > 0 {
		feeColor = color[0]
	}
	return &ContractFeesRecord{
		OwnerFee:     ownerFee,
		ValidatorFee: validatorFee,
		FeeColor:     feeColor,
	}
}

func ContractFeesRecordFromMarshalUtil(mu *marshalutil.MarshalUtil) (*ContractFeesRecord, error) {
	ret := &ContractFeesRecord{}
	var err error
	if ret.OwnerFee, err = mu.ReadUint64(); err != nil {
		return nil, err
	}
	if ret.ValidatorFee, err = mu.ReadUint64(); err != nil {
		return nil, err
	}
	if ret.FeeColor, err = colored.ColorFromBytes(mu.ReadRemainingBytes()); err != nil {
		return nil, err
	}
	return ret, nil
}

func (p *ContractFeesRecord) Bytes() []byte {
	mu := marshalutil.New()
	mu.WriteUint64(p.OwnerFee)
	mu.WriteUint64(p.ValidatorFee)
	mu.WriteBytes(p.FeeColor.Bytes())
	return mu.Bytes()
}

func ContractFeesRecordFromBytes(data []byte) (*ContractFeesRecord, error) {
	return ContractFeesRecordFromMarshalUtil(marshalutil.New(data))
}
