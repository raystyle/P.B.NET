package rsa

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_RSA(t *testing.T) {
	privatekey, err := Generate_Key(2048)
	require.Nil(t, err, err)
	file, err := ioutil.ReadFile("rsa.key")
	require.Nil(t, err, err)
	_, err = Import_PrivateKey_PEM(file)
	require.Nil(t, err, err)
	_, err = Import_PrivateKey_PEM(nil)
	require.Equal(t, err, ERR_INVALID_PEM_BLOCK, err)
	privatekey_bytes := Export_PrivateKey(privatekey)
	_, err = Import_PrivateKey(privatekey_bytes)
	require.Nil(t, err, err)
	publickey := &privatekey.PublicKey
	publickey_bytes := Export_PublicKey(publickey)
	_, err = Import_PublicKey(publickey_bytes)
	require.Nil(t, err, err)
	_, err = Import_PublicKey(nil)
	require.NotNil(t, err)
	signature, err := Sign(privatekey, file)
	require.Nil(t, err, err)
	require.True(t, Verify(publickey, file, signature), "invalid data")
	require.False(t, Verify(publickey, file[:10], signature), "error verify")
	require.False(t, Verify(publickey, file[:10], nil), "error verify")
	data := bytes.Repeat([]byte{0}, 128)
	cipherdata, err := Encrypt(publickey, data)
	require.Nil(t, err, err)
	plaindata, err := Decrypt(privatekey, cipherdata)
	require.Nil(t, err, err)
	require.Equal(t, plaindata, data, "wrong data")
}

func Benchmark_Sign(b *testing.B) {
	privatekey, err := Generate_Key(2048)
	require.Nil(b, err, err)
	data := bytes.Repeat([]byte{0}, 4096)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Sign(privatekey, data)
		// _, err := Sign(privatekey, data)
		// require.Nil(b, err, err)
	}
}

func Benchmark_Verify(b *testing.B) {
	privatekey, err := Generate_Key(2048)
	require.Nil(b, err, err)
	data := bytes.Repeat([]byte{0}, 256)
	signature, err := Sign(privatekey, data)
	require.Nil(b, err, err)
	publickey := &privatekey.PublicKey
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Verify(publickey, data, signature)
		// require.True(b, Verify(publickey, data, signature), "verify failed")
	}
	b.StopTimer()
}

func Benchmark_Sign_Verify(b *testing.B) {
	privatekey, err := Generate_Key(2048)
	require.Nil(b, err, err)
	data := bytes.Repeat([]byte{0}, 256)
	publickey := &privatekey.PublicKey
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		signature, _ := Sign(privatekey, data)
		Verify(publickey, data, signature)
		// signature, err := Sign(privatekey, data)
		// require.Nil(b, err, err)
		// require.True(b, Verify(publickey, data, signature), "verify failed")
	}
	b.StopTimer()
}
