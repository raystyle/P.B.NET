package aes

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_AES(t *testing.T) {
	key_128 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 11, 12, 13, 14, 15, 16}
	key_256 := bytes.Repeat(key_128, 2)
	iv := []byte{11, 12, 13, 14, 15, 16, 17, 18, 19, 10, 111, 112, 113, 114, 115, 116}
	data := bytes.Repeat([]byte{0}, 32)
	//encrypt&&decrypt
	f := func(key []byte) {
		cipherdata, err := CBC_Encrypt(data, key, iv)
		require.Nil(t, err, err)
		t.Log(cipherdata)
		plaindata, err := CBC_Decrypt(cipherdata, key, iv)
		require.Nil(t, err, err)
		require.Equal(t, plaindata, data, "wrong data")
	}
	f(key_128)
	f(key_256)
	//no data
	_, err := CBC_Encrypt(nil, key_128, iv)
	require.Equal(t, err, ERR_NO_DATA, err)
	_, err = CBC_Decrypt(nil, key_128, iv)
	require.Equal(t, err, ERR_NO_DATA, err)
	//invalid iv
	_, err = CBC_Encrypt(data, key_128, nil)
	require.Equal(t, err, ERR_INVALID_IV_SIZE, err)
	_, err = CBC_Decrypt(data, key_128, nil)
	require.Equal(t, err, ERR_INVALID_IV_SIZE, err)
	//invalid key
	_, err = CBC_Encrypt(data, nil, iv)
	require.NotNil(t, err)
	_, err = CBC_Decrypt(data, nil, iv)
	require.NotNil(t, err)
	//invalid data ERR_INVALID_CIPHERDATA
	_, err = CBC_Decrypt(bytes.Repeat([]byte{0}, 13), key_128, iv)
	require.Equal(t, err, ERR_INVALID_CIPHERDATA, err)
	_, err = CBC_Decrypt(bytes.Repeat([]byte{0}, 63), key_128, iv)
	require.Equal(t, err, ERR_INVALID_CIPHERDATA, err)
	//invalid data ERR_UNPADDING
	_, err = CBC_Decrypt(bytes.Repeat([]byte{0}, 64), key_128, iv)
	require.Equal(t, err, ERR_UNPADDING, err)
}

func Test_CBC_Cryptor(t *testing.T) {
	key_128 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 11, 12, 13, 14, 15, 16}
	key_256 := bytes.Repeat(key_128, 2)
	iv := []byte{11, 12, 13, 14, 15, 16, 17, 18, 19, 10, 111, 112, 113, 114, 115, 116}
	_, err := New_CBC_Cryptor(bytes.Repeat([]byte{0}, BIT128), iv)
	require.Nil(t, err, err)
	_, err = New_CBC_Cryptor(bytes.Repeat([]byte{0}, BIT192), iv)
	require.Nil(t, err, err)
	_, err = New_CBC_Cryptor(bytes.Repeat([]byte{0}, BIT256), iv)
	require.Nil(t, err, err)
	data := bytes.Repeat([]byte{0}, 32)
	//encrypt&&decrypt
	f := func(key []byte) {
		cryptor, err := New_CBC_Cryptor(key, iv)
		require.Nil(t, err, err)
		ff := func() {
			cipherdata, err := cryptor.Encrypt(data)
			require.Nil(t, err, err)
			t.Log(cipherdata)
			plaindata, err := cryptor.Decrypt(cipherdata)
			require.Nil(t, err, err)
			require.Equal(t, plaindata, data, "wrong data")
		}
		//x2
		ff()
		ff()
	}
	f(key_128)
	f(key_256)
	//invalid key
	cryptor, err := New_CBC_Cryptor(nil, iv)
	require.NotNil(t, err)
	//invalid iv
	cryptor, err = New_CBC_Cryptor(key_128, nil)
	require.NotNil(t, err)
	//no data
	cryptor, err = New_CBC_Cryptor(key_128, iv)
	require.Nil(t, err, err)
	_, err = cryptor.Encrypt(nil)
	require.Equal(t, err, ERR_NO_DATA, err)
	_, err = cryptor.Decrypt(nil)
	require.Equal(t, err, ERR_NO_DATA, err)
	//invalid data
	_, err = cryptor.Decrypt(bytes.Repeat([]byte{0}, 13))
	require.Equal(t, err, ERR_INVALID_CIPHERDATA, err)
	_, err = cryptor.Decrypt(bytes.Repeat([]byte{0}, 63))
	require.Equal(t, err, ERR_INVALID_CIPHERDATA, err)
	_, err = cryptor.Decrypt(bytes.Repeat([]byte{0}, 64))
	require.Equal(t, err, ERR_UNPADDING, err)
	cryptor.Key_IV()
}

func Benchmark_CBC_Encrypt_128(b *testing.B) {
	key := bytes.Repeat([]byte{0}, 16)
	data := bytes.Repeat([]byte{0}, 64)
	benchmark_cbc_encrypt(b, data, key)
}

func Benchmark_CBC_Encrypt_256(b *testing.B) {
	key := bytes.Repeat([]byte{0}, 32)
	data := bytes.Repeat([]byte{0}, 64)
	benchmark_cbc_encrypt(b, data, key)
}

func benchmark_cbc_encrypt(b *testing.B, data, key []byte) {
	iv := bytes.Repeat([]byte{0}, IV_SIZE)
	cryptor, err := New_CBC_Cryptor(key, iv)
	require.Nil(b, err, err)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cryptor.Encrypt(data)
	}
	b.StopTimer()
}

func Benchmark_CBC_Decrypt_128(b *testing.B) {
	key := bytes.Repeat([]byte{0}, 16)
	iv := bytes.Repeat([]byte{0}, IV_SIZE)
	cipherdata, err := CBC_Encrypt(bytes.Repeat([]byte{0}, 64), key, iv)
	require.Nil(b, err, err)
	benchmark_cbc_decrypt(b, cipherdata, key, iv)
}

func Benchmark_CBC_Decrypt_256(b *testing.B) {
	key := bytes.Repeat([]byte{0}, 32)
	iv := bytes.Repeat([]byte{0}, IV_SIZE)
	cipherdata, err := CBC_Encrypt(bytes.Repeat([]byte{0}, 64), key, iv)
	require.Nil(b, err, err)
	benchmark_cbc_decrypt(b, cipherdata, key, iv)
}

func benchmark_cbc_decrypt(b *testing.B, data, key, iv []byte) {
	cryptor, err := New_CBC_Cryptor(key, iv)
	require.Nil(b, err, err)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cryptor.Decrypt(data)
	}
	b.StopTimer()
}
