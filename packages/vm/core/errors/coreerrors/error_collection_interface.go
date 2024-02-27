package coreerrors

import (
	"github.com/iotaledger/wasp/packages/isc"
)

type ErrorCollection interface {
	Get(errorID uint16) (*isc.VMErrorTemplate, bool)
}

type ErrorCollectionWriter interface {
	ErrorCollection
	Register(messageFormat string) (*isc.VMErrorTemplate, error)
}
