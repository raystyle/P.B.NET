package light

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testutil"
)

func TestCrypto(t *testing.T) {
	testdata := testutil.GenerateData()
	c := newCrypto(nil)
	cipherData := c.encrypt(testdata)
	require.NotEqual(t, testdata, cipherData)
	c.decrypt(cipherData)
	require.Equal(t, testdata, cipherData)
	// has encrypt
	c = newCrypto(nil)
	key := make([]byte, 256)
	for i := 0; i < 256; i++ {
		key[i] = c[0][i]
	}
	c = newCrypto(key)
	cipherData = c.encrypt(testdata)
	require.NotEqual(t, testdata, cipherData)
	c.decrypt(cipherData)
	require.Equal(t, testdata, cipherData)
}

func BenchmarkCrypto_encrypt_512(b *testing.B) {
	benchmarkCryptoEncrypt(b, make([]byte, 512))
}

func BenchmarkCrypto_encrypt_1024(b *testing.B) {
	benchmarkCryptoEncrypt(b, make([]byte, 1024))
}

func BenchmarkCrypto_encrypt_4096(b *testing.B) {
	benchmarkCryptoEncrypt(b, make([]byte, 4096))
}

func benchmarkCryptoEncrypt(b *testing.B, testdata []byte) {
	c := newCrypto(nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.encrypt(testdata)
	}
	b.StopTimer()
}

func BenchmarkCrypto_decrypt_512(b *testing.B) {
	benchmarkCryptoDecrypt(b, make([]byte, 512))
}

func BenchmarkCrypto_decrypt_1024(b *testing.B) {
	benchmarkCryptoDecrypt(b, make([]byte, 1024))
}

func BenchmarkCrypto_decrypt_4096(b *testing.B) {
	benchmarkCryptoDecrypt(b, make([]byte, 4096))
}

func benchmarkCryptoDecrypt(b *testing.B, testdata []byte) {
	c := newCrypto(nil)
	cipherData := c.encrypt(testdata)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.decrypt(cipherData)
	}
	b.StopTimer()
}
