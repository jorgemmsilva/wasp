package gas

import (
	"fmt"
	"io"
	"math/big"

	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/util/rwutil"
)

type GasUnits uint64

// By default each token pays for 100 units of gas
var DefaultGasPerToken = util.Ratio32{A: 100, B: 1}

// GasPerToken + ValidatorFeeShare + EVMGasRatio
const FeePolicyByteSize = util.RatioByteSize + serializer.OneByte + util.RatioByteSize

type FeePolicy struct {
	// EVMGasRatio expresses the ratio at which EVM gas is converted to ISC gas
	// X = ISC gas, Y = EVM gas => ISC gas = EVM gas * A/B
	EVMGasRatio util.Ratio32 `json:"evmGasRatio" swagger:"desc(The EVM gas ratio (ISC gas = EVM gas * A/B)),required"`

	// GasPerToken specifies how many gas units are paid for each token.
	GasPerToken util.Ratio32 `json:"gasPerToken" swagger:"desc(The gas per token ratio (A/B) (gas/token)),required"`

	// ValidatorFeeShare Validator/Governor fee split: percentage of fees which goes to Validator
	// 0 mean all goes to Governor
	// >=100 all goes to Validator
	ValidatorFeeShare uint8 `json:"validatorFeeShare" swagger:"desc(The validator fee share.),required"`
}

// FeeFromGasBurned calculates the how many tokens to take and where
// to deposit them.
func (p *FeePolicy) FeeFromGasBurned(gasUnits GasUnits, availableTokens iotago.BaseToken) (sendToOwner, sendToValidator iotago.BaseToken) {
	var fee iotago.BaseToken

	// round up
	fee = p.FeeFromGas(gasUnits)
	fee = min(fee, availableTokens)

	validatorPercentage := p.ValidatorFeeShare
	if validatorPercentage > 100 {
		validatorPercentage = 100
	}
	// safe arithmetics
	if fee >= 100 {
		sendToValidator = (fee / 100) * iotago.BaseToken(validatorPercentage)
	} else {
		sendToValidator = (fee * iotago.BaseToken(validatorPercentage)) / 100
	}
	return fee - sendToValidator, sendToValidator
}

func FeeFromGas(gasUnits GasUnits, gasPerToken util.Ratio32) iotago.BaseToken {
	return iotago.BaseToken(gasPerToken.YCeil64(uint64(gasUnits)))
}

func (p *FeePolicy) FeeFromGas(gasUnits GasUnits) iotago.BaseToken {
	return FeeFromGas(gasUnits, p.GasPerToken)
}

func (p *FeePolicy) MinFee() iotago.BaseToken {
	if p.GasPerToken.A == 0 {
		return 0
	}
	return p.FeeFromGas(BurnCodeMinimumGasPerRequest1P.Cost())
}

func (p *FeePolicy) IsEnoughForMinimumFee(availableTokens iotago.BaseToken) bool {
	return availableTokens >= p.MinFee()
}

// if GasPerToken is '0:0' then set the GasBudget to MaxGasPerRequest
func (p *FeePolicy) GasBudgetFromTokens(availableTokens iotago.BaseToken, limits ...*Limits) GasUnits {
	if p.GasPerToken.IsZero() {
		if len(limits) == 0 {
			panic("GasBudgetFromTokens without giving limits when GasPerToken")
		}
		return limits[0].MaxGasPerRequest
	}
	return GasUnits(p.GasPerToken.XFloor64(uint64(availableTokens)))
}

func DefaultFeePolicy() *FeePolicy {
	return &FeePolicy{
		GasPerToken:       DefaultGasPerToken,
		ValidatorFeeShare: 0, // by default all goes to the governor
		EVMGasRatio:       DefaultEVMGasRatio,
	}
}

func MustFeePolicyFromBytes(data []byte) *FeePolicy {
	ret, err := FeePolicyFromBytes(data)
	if err != nil {
		panic(err)
	}
	return ret
}

func FeePolicyFromBytes(data []byte) (*FeePolicy, error) {
	return rwutil.ReadFromBytes(data, new(FeePolicy))
}

func (p *FeePolicy) Bytes() []byte {
	return rwutil.WriteToBytes(p)
}

func (p *FeePolicy) String() string {
	return fmt.Sprintf(`
	GasPerToken %s
	EVMGasRatio %s
	ValidatorFeeShare %d
	`,
		p.GasPerToken,
		p.EVMGasRatio,
		p.ValidatorFeeShare,
	)
}

func (p *FeePolicy) Read(r io.Reader) error {
	rr := rwutil.NewReader(r)
	rr.Read(&p.EVMGasRatio)
	rr.Read(&p.GasPerToken)
	p.ValidatorFeeShare = rr.ReadUint8()
	return rr.Err
}

func (p *FeePolicy) Write(w io.Writer) error {
	ww := rwutil.NewWriter(w)
	ww.Write(&p.EVMGasRatio)
	ww.Write(&p.GasPerToken)
	ww.WriteUint8(p.ValidatorFeeShare)
	return ww.Err
}

// GasPriceWei returns the gas price converted to wei
func (p *FeePolicy) GasPriceWei(l1BaseTokenDecimals uint32) *big.Int {
	// special case '0:0' for free request
	if p.GasPerToken.IsZero() {
		return big.NewInt(0)
	}

	// convert to wei (18 decimals)
	decimalsDifference := 18 - l1BaseTokenDecimals
	price := big.NewInt(10)
	price.Exp(price, new(big.Int).SetUint64(uint64(decimalsDifference)), nil)

	price.Mul(price, new(big.Int).SetUint64(uint64(p.GasPerToken.B)))
	price.Div(price, new(big.Int).SetUint64(uint64(p.GasPerToken.A)))
	price.Mul(price, new(big.Int).SetUint64(uint64(p.EVMGasRatio.A)))
	price.Div(price, new(big.Int).SetUint64(uint64(p.EVMGasRatio.B)))

	return price
}
