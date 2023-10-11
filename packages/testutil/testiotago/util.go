package testiotago

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/tpkg"
)

func RandNativeTokenID() (ret iotago.NativeTokenID) {
	return tpkg.RandNativeToken().ID
}

func RandOutputID() iotago.OutputID {
	return tpkg.RandOutputID(tpkg.RandUint16(10))
}

func RandAliasID() (ret iotago.AccountID) {
	return tpkg.RandAccountAddress().AccountID()
}
