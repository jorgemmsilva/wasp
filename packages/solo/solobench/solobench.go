// package solobench provides tools to benchmark contracts running under solo
package solobench

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/util"
)

type Func func(b *testing.B, chain *solo.Chain, reqs []*solo.CallParams, keyPair *cryptolib.KeyPair)

// RunBenchmarkSync processes requests synchronously, producing 1 block per request
func RunBenchmarkSync(b *testing.B, chain *solo.Chain, reqs []*solo.CallParams, keyPair *cryptolib.KeyPair) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := chain.PostRequestSync(reqs[i], keyPair)
		require.NoError(b, err)
	}
}

// RunBenchmarkAsync processes requests asynchronously, producing 1 block per many requests
func RunBenchmarkAsync(b *testing.B, chain *solo.Chain, reqs []*solo.CallParams, keyPair *cryptolib.KeyPair) {
	_ = keyPair
	blocks := make([]*iotago.Block, b.N)
	for i := 0; i < b.N; i++ {
		var err error
		blocks[i], _, err = chain.RequestFromParamsToLedger(reqs[i], nil)
		require.NoError(b, err)
	}

	chain.WaitForRequestsMark()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		go chain.Env.EnqueueRequests(util.TxFromBlock(blocks[i]))
	}
	require.True(b, chain.WaitForRequestsThrough(b.N, 20*time.Second))
}
