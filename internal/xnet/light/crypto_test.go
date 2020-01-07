package light

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestCrypto(t *testing.T) {
	testdata := testsuite.Bytes()
	c := newCrypto(nil)
	cipherData := c.Encrypt(testdata)
	require.NotEqual(t, testdata, cipherData)
	c.Decrypt(cipherData)
	require.Equal(t, testdata, cipherData)
	// has encrypt
	c = newCrypto(nil)
	key := make([]byte, 256)
	for i := 0; i < 256; i++ {
		key[i] = c[0][i]
	}
	c = newCrypto(key)
	cipherData = c.Encrypt(testdata)
	require.NotEqual(t, testdata, cipherData)
	c.Decrypt(cipherData)
	require.Equal(t, testdata, cipherData)
}

func BenchmarkCrypto_Encrypt_512(b *testing.B) {
	benchmarkCryptoEncrypt(b, make([]byte, 512))
}

func BenchmarkCrypto_Encrypt_1024(b *testing.B) {
	benchmarkCryptoEncrypt(b, make([]byte, 1024))
}

func BenchmarkCrypto_Encrypt_4096(b *testing.B) {
	benchmarkCryptoEncrypt(b, make([]byte, 4096))
}

func benchmarkCryptoEncrypt(b *testing.B, testdata []byte) {
	c := newCrypto(nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Encrypt(testdata)
	}
	b.StopTimer()
}

func BenchmarkCrypto_Decrypt_512(b *testing.B) {
	benchmarkCryptoDecrypt(b, make([]byte, 512))
}

func BenchmarkCrypto_Decrypt_1024(b *testing.B) {
	benchmarkCryptoDecrypt(b, make([]byte, 1024))
}

func BenchmarkCrypto_Decrypt_4096(b *testing.B) {
	benchmarkCryptoDecrypt(b, make([]byte, 4096))
}

func benchmarkCryptoDecrypt(b *testing.B, testdata []byte) {
	c := newCrypto(nil)
	cipherData := c.Encrypt(testdata)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Decrypt(cipherData)
	}
	b.StopTimer()
}
