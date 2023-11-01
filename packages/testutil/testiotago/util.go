package testiotago

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/tpkg"
)

func RandNativeTokenID() (ret iotago.NativeTokenID) {
	return tpkg.RandNativeTokenID()
}

func RandOutputID() iotago.OutputID {
	return tpkg.RandOutputID(tpkg.RandUint16(10))
}

func RandAnchorID() (ret iotago.AnchorID) {
	return tpkg.RandAnchorAddress().AnchorID()
}

func RandAccountID() (ret iotago.AccountID) {
	return tpkg.RandAccountAddress().AccountID()
}
