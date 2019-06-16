package light

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_cryptor(t *testing.T) {
	testdata := make([]byte, 256)
	for i := 0; i < 256; i++ {
		testdata[i] = byte(i)
	}
	c := new_cryptor(nil)
	cipherdata := c.encrypt(testdata)
	require.NotEqual(t, testdata, cipherdata)
	c.decrypt(cipherdata)
	require.Equal(t, testdata, cipherdata)
	// has encrypt
	c = new_cryptor(nil)
	key := make([]byte, 256)
	for i := 0; i < 256; i++ {
		key[i] = c[0][i]
	}
	c = new_cryptor(key)
	cipherdata = c.encrypt(testdata)
	require.NotEqual(t, testdata, cipherdata)
	c.decrypt(cipherdata)
	require.Equal(t, testdata, cipherdata)
}

func Benchmark_cryptor_encrypt_512(b *testing.B) {
	benchmark_cryptor_encrypt(b, make([]byte, 512))
}

func Benchmark_cryptor_encrypt_1024(b *testing.B) {
	benchmark_cryptor_encrypt(b, make([]byte, 1024))
}

func Benchmark_cryptor_encrypt_4096(b *testing.B) {
	benchmark_cryptor_encrypt(b, make([]byte, 4096))
}

func benchmark_cryptor_encrypt(b *testing.B, testdata []byte) {
	c := new_cryptor(nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.encrypt(testdata)
	}
	b.StopTimer()
}

func Benchmark_cryptor_decrypt_512(b *testing.B) {
	benchmark_cryptor_decrypt(b, make([]byte, 512))
}

func Benchmark_cryptor_decrypt_1024(b *testing.B) {
	benchmark_cryptor_decrypt(b, make([]byte, 1024))
}

func Benchmark_cryptor_decrypt_4096(b *testing.B) {
	benchmark_cryptor_decrypt(b, make([]byte, 4096))
}

func benchmark_cryptor_decrypt(b *testing.B, testdata []byte) {
	c := new_cryptor(nil)
	cipherdata := c.encrypt(testdata)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.decrypt(cipherdata)
	}
	b.StopTimer()
}
