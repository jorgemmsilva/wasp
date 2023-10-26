package sandbox

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/hexutil"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

type utilImpl struct {
	gas isc.Gas
}

func NewUtils(g isc.Gas) isc.Utils {
	return utilImpl{g}
}

// ------ isc.Utils() interface

func (u utilImpl) Hashing() isc.Hashing {
	return u
}

func (u utilImpl) ED25519() isc.ED25519 {
	return u
}

// --- isc.Hex interface

func (u utilImpl) Decode(s string) ([]byte, error) {
	u.gas.Burn(gas.BurnCodeUtilsHexDecode)
	return hexutil.DecodeHex(s)
}

func (u utilImpl) Encode(data []byte) string {
	u.gas.Burn(gas.BurnCodeUtilsHexEncode)
	return hexutil.EncodeHex(data)
}

// --- isc.Hashing interface

func (u utilImpl) Blake2b(data []byte) hashing.HashValue {
	u.gas.Burn(gas.BurnCodeUtilsHashingBlake2b)
	return hashing.HashDataBlake2b(data)
}

func (u utilImpl) Hname(name string) isc.Hname {
	u.gas.Burn(gas.BurnCodeUtilsHashingHname)
	return isc.Hn(name)
}

func (u utilImpl) Keccak(data []byte) hashing.HashValue {
	// no need for a new burn code, since Keccak == SHA3 with different padding
	u.gas.Burn(gas.BurnCodeUtilsHashingSha3)
	return hashing.HashKeccak(data)
}

func (u utilImpl) Sha3(data []byte) hashing.HashValue {
	u.gas.Burn(gas.BurnCodeUtilsHashingSha3)
	return hashing.HashSha3(data)
}

// --- isc.ED25519 interface

func (u utilImpl) ValidSignature(data, pubKey, signature []byte) bool {
	u.gas.Burn(gas.BurnCodeUtilsED25519ValidSig)
	pk, err := cryptolib.PublicKeyFromBytes(pubKey)
	if err != nil {
		return false
	}
	sig, err := cryptolib.SignatureFromBytes(signature)
	if err != nil {
		return false
	}
	return pk.Verify(data, sig[:])
}

func (u utilImpl) AddressFromPublicKey(pubKey []byte) (iotago.Address, error) {
	u.gas.Burn(gas.BurnCodeUtilsED25519AddrFromPubKey)
	pk, err := cryptolib.PublicKeyFromBytes(pubKey)
	if err != nil {
		return nil, err
	}
	return pk.AsEd25519Address(), nil
}
