package rsa

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRSA(t *testing.T) {
	privatekey, err := GenerateKey(2048)
	require.NoError(t, err)
	file, err := ioutil.ReadFile("rsa.key")
	require.NoError(t, err)
	_, err = ImportPrivateKeyPEM(file)
	require.NoError(t, err)
	_, err = ImportPrivateKeyPEM(nil)
	require.Equal(t, err, ErrInvalidPEMBlock, err)
	privatekeyBytes := ExportPrivateKey(privatekey)
	_, err = ImportPrivateKey(privatekeyBytes)
	require.NoError(t, err)
	publickey := &privatekey.PublicKey
	publickeyBytes := ExportPublicKey(publickey)
	_, err = ImportPublicKey(publickeyBytes)
	require.NoError(t, err)
	_, err = ImportPublicKey(nil)
	require.NotNil(t, err)
	signature, err := Sign(privatekey, file)
	require.NoError(t, err)
	require.True(t, Verify(publickey, file, signature), "invalid data")
	require.False(t, Verify(publickey, file[:10], signature), "error verify")
	require.False(t, Verify(publickey, file[:10], nil), "error verify")
	data := bytes.Repeat([]byte{0}, 128)
	cipherdata, err := Encrypt(publickey, data)
	require.NoError(t, err)
	plaindata, err := Decrypt(privatekey, cipherdata)
	require.NoError(t, err)
	require.Equal(t, plaindata, data, "wrong data")
}

func BenchmarkSign(b *testing.B) {
	privatekey, err := GenerateKey(2048)
	require.NoError(b, err)
	data := bytes.Repeat([]byte{0}, 4096)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Sign(privatekey, data)
		// _, err := Sign(privatekey, data)
		// require.NoError(b, err)
	}
}

func BenchmarkVerify(b *testing.B) {
	privatekey, err := GenerateKey(2048)
	require.NoError(b, err)
	data := bytes.Repeat([]byte{0}, 256)
	signature, err := Sign(privatekey, data)
	require.NoError(b, err)
	publickey := &privatekey.PublicKey
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Verify(publickey, data, signature)
		// require.True(b, Verify(publickey, data, signature), "verify failed")
	}
	b.StopTimer()
}

func BenchmarkSignVerify(b *testing.B) {
	privatekey, err := GenerateKey(2048)
	require.NoError(b, err)
	data := bytes.Repeat([]byte{0}, 256)
	publickey := &privatekey.PublicKey
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		signature, _ := Sign(privatekey, data)
		Verify(publickey, data, signature)
		// signature, err := Sign(privatekey, data)
		// require.NoError(b, err)
		// require.True(b, Verify(publickey, data, signature), "verify failed")
	}
	b.StopTimer()
}
