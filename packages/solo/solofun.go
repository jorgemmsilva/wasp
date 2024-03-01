package solo

import (
	"math"

	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/testutil/testkey"
	"github.com/iotaledger/wasp/packages/testutil/utxodb"
)

func (env *Solo) NewKeyPairFromIndex(index int) *cryptolib.KeyPair {
	seed := env.NewSeedFromIndex(index)
	return cryptolib.KeyPairFromSeed(*seed)
}

func (env *Solo) NewSeedFromIndex(index int) *cryptolib.Seed {
	if index < 0 {
		// SubSeed takes a "uint31"
		index += math.MaxUint32 / 2
	}
	seed := cryptolib.SubSeed(env.seed[:], uint32(index), testutil.L1API.ProtocolParameters().Bech32HRP())
	return &seed
}

// NewSignatureSchemeWithFundsAndPubKey generates new ed25519 signature scheme
// and requests some tokens from the UTXODB faucet.
// The amount of tokens is equal to utxodb.FundsFromFaucetAmount (=1000Mi) base tokens
// Returns signature scheme interface and public key in binary form
func (env *Solo) NewKeyPairWithFunds(seed ...*cryptolib.Seed) (*cryptolib.KeyPair, iotago.Address) {
	env.ledgerMutex.Lock()
	defer env.ledgerMutex.Unlock()

	keyPair, addr := env.NewKeyPair(seed...)
	_, _, err := env.utxoDB.NewWalletWithFundsFromFaucet(keyPair)
	require.NoError(env.T, err)
	env.AssertL1BaseTokens(addr, utxodb.FundsFromFaucetAmount)

	return keyPair, addr
}

// NewSignatureSchemeAndPubKey generates new ed25519 signature scheme
// Returns signature scheme interface and public key in binary form
func (env *Solo) NewKeyPair(seedOpt ...*cryptolib.Seed) (*cryptolib.KeyPair, iotago.Address) {
	return testkey.GenKeyAddr(seedOpt...)
}
