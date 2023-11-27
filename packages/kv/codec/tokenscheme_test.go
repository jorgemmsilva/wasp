package codec

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
)

func TestTokenSchemeSerialization(t *testing.T) {
	ts := &iotago.SimpleTokenScheme{
		MintedTokens:  big.NewInt(1001),
		MeltedTokens:  big.NewInt(1002),
		MaximumSupply: big.NewInt(1003),
	}
	l1API := iotago.V3API(iotago.NewV3ProtocolParameters(iotago.WithVersion(3)))
	enc := EncodeTokenScheme(ts, l1API)
	tsBack, err := DecodeTokenScheme(enc, l1API)
	require.NoError(t, err)
	require.EqualValues(t, ts, tsBack)
}
