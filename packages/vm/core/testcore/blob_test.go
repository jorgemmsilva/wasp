package testcore

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/testutil/testdbhash"
	"github.com/iotaledger/wasp/packages/vm/core/blob"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

const (
	randomFile = "blob_test.go"
	wasmFile   = "sbtests/sbtestsc/testcore_bg.wasm"
)

func TestUploadBlob(t *testing.T) {
	t.Run("from binary", func(t *testing.T) {
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		ch := env.NewChain()

		ch.MustDepositBaseTokensToL2(100_000, nil)

		h, err := ch.UploadBlob(nil, dict.Dict{"field": []byte("dummy data")})
		require.NoError(t, err)

		_, ok := ch.GetBlobInfo(h)
		require.True(t, ok)

		testdbhash.VerifyContractStateHash(env, blob.Contract, "", t.Name())
	})
	t.Run("huge", func(t *testing.T) {
		env := solo.New(t)
		ch := env.NewChain()

		err := ch.DepositBaseTokensToL2(1_000_000, nil)
		require.NoError(t, err)

		limits := *gas.LimitsDefault
		limits.MaxGasPerRequest = 10 * limits.MaxGasPerRequest
		limits.MaxGasExternalViewCall = 10 * limits.MaxGasExternalViewCall
		ch.SetGasLimits(nil, &limits)
		ch.WaitForRequestsMark()

		size := int64(1 * 900 * 1024) // 900 KB
		randomData := make([]byte, size+1)
		_, err = rand.Read(randomData)
		require.NoError(t, err)
		h, err := ch.UploadBlob(nil, dict.Dict{"field": randomData})
		require.NoError(t, err)

		_, ok := ch.GetBlobInfo(h)
		require.True(t, ok)
	})
	t.Run("from file", func(t *testing.T) {
		env := solo.New(t)
		ch := env.NewChain()

		err := ch.DepositBaseTokensToL2(100_000, nil)
		require.NoError(t, err)

		h, err := ch.UploadBlobFromFile(nil, randomFile, "file")
		require.NoError(t, err)

		_, ok := ch.GetBlobInfo(h)
		require.True(t, ok)
	})
	t.Run("several", func(t *testing.T) {
		env := solo.New(t)
		ch := env.NewChain()

		err := ch.DepositBaseTokensToL2(100_000, nil)
		require.NoError(t, err)

		const howMany = 5
		hashes := make([]hashing.HashValue, howMany)
		for i := 0; i < howMany; i++ {
			data := []byte(fmt.Sprintf("dummy data #%d", i))
			hashes[i], err = ch.UploadBlob(nil, dict.Dict{"field": data})
			require.NoError(t, err)
			m, ok := ch.GetBlobInfo(hashes[i])
			require.True(t, ok)
			require.EqualValues(t, 1, len(m))
			require.EqualValues(t, len(data), m["field"])
		}
		blobList := func() map[hashing.HashValue]uint32 {
			ret, err := ch.CallView(blob.ViewListBlobs.Message())
			require.NoError(t, err)
			return lo.Must(blob.ViewListBlobs.Output.Decode(ret))
		}()
		require.EqualValues(t, howMany, len(blobList))
		for _, h := range hashes {
			size := blobList[h]
			require.EqualValues(t, len("dummy data #1"), size)

			ret2, err := ch.CallView(blob.ViewGetBlobField.Message(h, []byte("field")))
			require.NoError(t, err)
			require.EqualValues(t, 1, len(ret2))
			data := lo.Must(blob.ViewGetBlobField.Output.Decode(ret2))
			require.EqualValues(t, size, len(data))
		}
	})
}

func TestUploadWasm(t *testing.T) {
	t.Run("upload wasm", func(t *testing.T) {
		env := solo.New(t)
		ch := env.NewChain()
		ch.MustDepositBaseTokensToL2(100_000, nil)
		binary := []byte("supposed to be wasm")
		hwasm, err := ch.UploadWasm(nil, binary)
		require.NoError(t, err)

		binBack, err := ch.GetWasmBinary(hwasm)
		require.NoError(t, err)

		require.EqualValues(t, binary, binBack)
	})
	t.Run("upload twice", func(t *testing.T) {
		env := solo.New(t)
		ch := env.NewChain()
		ch.MustDepositBaseTokensToL2(100_000, nil)
		binary := []byte("supposed to be wasm")
		hwasm1, err := ch.UploadWasm(nil, binary)
		require.NoError(t, err)

		// we upload exactly the same, if it exists it silently returns no error
		hwasm2, err := ch.UploadWasm(nil, binary)
		require.NoError(t, err)

		require.EqualValues(t, hwasm1, hwasm2)

		binBack, err := ch.GetWasmBinary(hwasm1)
		require.NoError(t, err)

		require.EqualValues(t, binary, binBack)
	})
	t.Run("upload wasm from file", func(t *testing.T) {
		t.SkipNow() // TODO: wasm
		env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
		ch := env.NewChain()
		ch.MustDepositBaseTokensToL2(100_000, nil)
		progHash, err := ch.UploadWasmFromFile(nil, wasmFile)
		require.NoError(t, err)

		err = ch.DeployContract(nil, "testCore", progHash, nil)
		require.NoError(t, err)
	})
	t.Run("list blobs", func(t *testing.T) {
		env := solo.New(t)
		ch := env.NewChain()
		ch.MustDepositBaseTokensToL2(100_000, nil)
		_, err := ch.UploadWasmFromFile(nil, wasmFile)
		require.NoError(t, err)

		ret, err := ch.CallView(blob.ViewListBlobs.Message())
		require.NoError(t, err)
		require.EqualValues(t, 1, len(ret))
	})
}

func TestBigBlob(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustStorageDeposit: true})
	ch := env.NewChain()
	ch.MustDepositBaseTokensToL2(1*isc.Million, nil)

	upload := func(n int) gas.GasUnits {
		blobBin := make([]byte, n)
		_, err := ch.UploadWasm(ch.OriginatorPrivateKey, blobBin)
		require.NoError(t, err)
		return ch.LastReceipt().GasBurned
	}

	gas1k := upload(100_000)
	gas2k := upload(200_000)

	t.Log(gas1k, gas2k)
	require.Greater(t, gas2k, gas1k)
}
