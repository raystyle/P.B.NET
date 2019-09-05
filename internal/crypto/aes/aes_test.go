package aes

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAES(t *testing.T) {
	key128 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 11, 12, 13, 14, 15, 16}
	key256 := bytes.Repeat(key128, 2)
	iv := []byte{11, 12, 13, 14, 15, 16, 17, 18, 19, 10, 111, 112, 113, 114, 115, 116}
	data := bytes.Repeat([]byte{0}, 32)
	// encrypt&&decrypt
	f := func(key []byte) {
		cipherdata, err := CBCEncrypt(data, key, iv)
		require.NoError(t, err)
		t.Log(cipherdata)
		plaindata, err := CBCDecrypt(cipherdata, key, iv)
		require.NoError(t, err)
		require.Equal(t, plaindata, data)
	}
	f(key128)
	f(key256)
	// no data
	_, err := CBCEncrypt(nil, key128, iv)
	require.Equal(t, err, ErrNoData)
	_, err = CBCDecrypt(nil, key128, iv)
	require.Equal(t, err, ErrNoData)
	// invalid iv
	_, err = CBCEncrypt(data, key128, nil)
	require.Equal(t, err, ErrInvalidIVSize)
	_, err = CBCDecrypt(data, key128, nil)
	require.Equal(t, err, ErrInvalidIVSize)
	// invalid key
	_, err = CBCEncrypt(data, nil, iv)
	require.Error(t, err)
	_, err = CBCDecrypt(data, nil, iv)
	require.Error(t, err)
	// invalid data ErrInvalidCipherData
	_, err = CBCDecrypt(bytes.Repeat([]byte{0}, 13), key128, iv)
	require.Equal(t, err, ErrInvalidCipherData)
	_, err = CBCDecrypt(bytes.Repeat([]byte{0}, 63), key128, iv)
	require.Equal(t, err, ErrInvalidCipherData)
	// invalid data ErrUnpadding
	_, err = CBCDecrypt(bytes.Repeat([]byte{0}, 64), key128, iv)
	require.Equal(t, err, ErrUnpadding)
}

func TestCBC(t *testing.T) {
	key128 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 11, 12, 13, 14, 15, 16}
	key256 := bytes.Repeat(key128, 2)
	iv := []byte{11, 12, 13, 14, 15, 16, 17, 18, 19, 10, 111, 112, 113, 114, 115, 116}
	_, err := NewCBC(bytes.Repeat([]byte{0}, Bit128), iv)
	require.NoError(t, err)
	_, err = NewCBC(bytes.Repeat([]byte{0}, Bit192), iv)
	require.NoError(t, err)
	_, err = NewCBC(bytes.Repeat([]byte{0}, Bit256), iv)
	require.NoError(t, err)
	data := bytes.Repeat([]byte{0}, 32)
	// encrypt&&decrypt
	f := func(key []byte) {
		cbc, err := NewCBC(key, iv)
		require.NoError(t, err)
		cipherdata, err := cbc.Encrypt(data)
		require.NoError(t, err)
		t.Log(cipherdata)
		cipherdata, err = cbc.Encrypt(data)
		require.NoError(t, err)
		t.Log(cipherdata)
		plaindata, err := cbc.Decrypt(cipherdata)
		require.NoError(t, err)
		require.Equal(t, plaindata, data)
		plaindata, err = cbc.Decrypt(cipherdata)
		require.NoError(t, err)
		require.Equal(t, plaindata, data)
	}
	f(key128)
	f(key256)
	// invalid key
	cbc, err := NewCBC(nil, iv)
	require.Error(t, err)
	// invalid iv
	cbc, err = NewCBC(key128, nil)
	require.Error(t, err)
	// no data
	cbc, err = NewCBC(key128, iv)
	require.NoError(t, err)
	_, err = cbc.Encrypt(nil)
	require.Equal(t, err, ErrNoData)
	_, err = cbc.Decrypt(nil)
	require.Equal(t, err, ErrNoData)
	// invalid data
	_, err = cbc.Decrypt(bytes.Repeat([]byte{0}, 13))
	require.Equal(t, err, ErrInvalidCipherData)
	_, err = cbc.Decrypt(bytes.Repeat([]byte{0}, 63))
	require.Equal(t, err, ErrInvalidCipherData)
	_, err = cbc.Decrypt(bytes.Repeat([]byte{0}, 64))
	require.Equal(t, err, ErrUnpadding)
	// key iv
	k, v := cbc.KeyIV()
	require.Equal(t, key128, k)
	require.Equal(t, iv, v)
}

func BenchmarkCBC_Encrypt_128(b *testing.B) {
	key := bytes.Repeat([]byte{0}, 16)
	data := bytes.Repeat([]byte{0}, 64)
	benchmarkCBCEncrypt(b, data, key)
}

func BenchmarkCBC_Encrypt_256(b *testing.B) {
	key := bytes.Repeat([]byte{0}, 32)
	data := bytes.Repeat([]byte{0}, 64)
	benchmarkCBCEncrypt(b, data, key)
}

func benchmarkCBCEncrypt(b *testing.B, data, key []byte) {
	iv := bytes.Repeat([]byte{0}, IVSize)
	cbc, err := NewCBC(key, iv)
	require.NoError(b, err)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cbc.Encrypt(data)
	}
	b.StopTimer()
}

func BenchmarkCBC_Decrypt_128(b *testing.B) {
	key := bytes.Repeat([]byte{0}, 16)
	iv := bytes.Repeat([]byte{0}, IVSize)
	cipherdata, err := CBCEncrypt(bytes.Repeat([]byte{0}, 64), key, iv)
	require.NoError(b, err)
	benchmarkCBCDecrypt(b, cipherdata, key, iv)
}

func BenchmarkCBC_Decrypt_256(b *testing.B) {
	key := bytes.Repeat([]byte{0}, 32)
	iv := bytes.Repeat([]byte{0}, IVSize)
	cipherdata, err := CBCEncrypt(bytes.Repeat([]byte{0}, 64), key, iv)
	require.NoError(b, err)
	benchmarkCBCDecrypt(b, cipherdata, key, iv)
}

func benchmarkCBCDecrypt(b *testing.B, data, key, iv []byte) {
	c, err := NewCBC(key, iv)
	require.NoError(b, err)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Decrypt(data)
	}
	b.StopTimer()
}
