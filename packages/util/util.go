package util

import (
	"errors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/mr-tron/base58"
)

func ColorFromString(cs string) (ret ledgerstate.Color, err error) {
	if cs == "IOTA" {
		ret = ledgerstate.ColorIOTA
		return
	}
	var bin []byte
	bin, err = base58.Decode(cs)
	if err != nil {
		return
	}
	ret, err = ColorFromBytes(bin)
	return
}

func ColorFromBytes(cb []byte) (ret ledgerstate.Color, err error) {
	if len(cb) != ledgerstate.ColorLength {
		err = errors.New("must be exactly 32 bytes for color")
		return
	}
	copy(ret[:], cb)
	return
}
