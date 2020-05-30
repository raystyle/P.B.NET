package light

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestCrypto(t *testing.T) {
	testdata := testsuite.Bytes()
	crypto := newCrypto(nil)

	cipherData := crypto.Encrypt(testdata)
	require.NotEqual(t, testdata, cipherData)

	crypto.Decrypt(cipherData)
	require.Equal(t, testdata, cipherData)

	// has encrypt
	crypto = newCrypto(nil)
	key := make([]byte, 256)
	for i := 0; i < 256; i++ {
		key[i] = crypto[0][i]
	}
	crypto = newCrypto(key)

	cipherData = crypto.Encrypt(testdata)
	require.NotEqual(t, testdata, cipherData)

	crypto.Decrypt(cipherData)
	require.Equal(t, testdata, cipherData)
}

func BenchmarkCrypto_Encrypt(b *testing.B) {
	for _, size := range []int{
		128, 2048, 4096, 32768, 1048576,
	} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			benchmarkCryptoEncrypt(b, size)
		})
	}
}

func benchmarkCryptoEncrypt(b *testing.B, size int) {
	testdata := make([]byte, size)
	crypto := newCrypto(nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		crypto.Encrypt(testdata)
	}

	b.StopTimer()
}

func BenchmarkCrypto_Decrypt(b *testing.B) {
	for _, size := range []int{
		128, 2048, 4096, 32768, 1048576,
	} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			benchmarkCryptoDecrypt(b, size)
		})
	}
}

func benchmarkCryptoDecrypt(b *testing.B, size int) {
	testdata := make([]byte, size)
	crypto := newCrypto(nil)
	cipherData := crypto.Encrypt(testdata)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		crypto.Decrypt(cipherData)
	}

	b.StopTimer()
}
