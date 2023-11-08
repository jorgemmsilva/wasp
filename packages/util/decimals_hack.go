package util

import (
	"math/big"

	iotago "github.com/iotaledger/iota.go/v4"
)

const ethereumDecimals = uint32(18)

func adaptDecimals(value *big.Int, fromDecimals, toDecimals uint32) (result *big.Int, remainder *big.Int) {
	result = new(big.Int)
	remainder = new(big.Int)
	exp := big.NewInt(10)
	if toDecimals > fromDecimals {
		exp.Exp(exp, big.NewInt(int64(toDecimals-fromDecimals)), nil)
		result.Mul(value, exp)
	} else {
		exp.Exp(exp, big.NewInt(int64(fromDecimals-toDecimals)), nil)
		result.DivMod(value, exp, remainder)
	}
	return
}

// wei => base tokens
func EthereumDecimalsToBaseTokenDecimals(value *big.Int, baseTokenDecimals uint32) (result iotago.BaseToken, remainder *big.Int) {
	r, m := adaptDecimals(value, ethereumDecimals, baseTokenDecimals)
	if !r.IsUint64() {
		panic("cannot convert ether value to base tokens: too large")
	}
	return iotago.BaseToken(r.Uint64()), m
}

func MustEthereumDecimalsToBaseTokenDecimalsExact(value *big.Int, baseTokenDecimals uint32) (result iotago.BaseToken) {
	r, m := EthereumDecimalsToBaseTokenDecimals(value, baseTokenDecimals)
	if m.Sign() != 0 {
		panic("cannot convert ether value to base tokens: non-exact conversion")
	}
	return r
}

// base tokens => wei
func BaseTokensDecimalsToEthereumDecimals(value iotago.BaseToken, baseTokenDecimals uint32) (result *big.Int, remainder iotago.BaseToken) {
	r, m := adaptDecimals(new(big.Int).SetUint64(uint64(value)), baseTokenDecimals, ethereumDecimals)
	if !m.IsUint64() {
		panic("cannot convert ether value to base tokens: too large")
	}
	return r, iotago.BaseToken(m.Uint64())
}

func MustBaseTokensDecimalsToEthereumDecimalsExact(value iotago.BaseToken, baseTokenDecimals uint32) (result *big.Int) {
	r, m := BaseTokensDecimalsToEthereumDecimals(value, baseTokenDecimals)
	if m != 0 {
		panic("cannot convert base tokens value to ether: non-exact conversion")
	}
	return r
}
