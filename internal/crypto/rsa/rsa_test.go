package rsa

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRSA(t *testing.T) {
	privateKey, err := GenerateKey(2048)
	require.NoError(t, err)
	file, err := ioutil.ReadFile("rsa.key")
	require.NoError(t, err)
	_, err = ImportPrivateKeyPEM(file)
	require.NoError(t, err)
	_, err = ImportPrivateKeyPEM(nil)
	require.Equal(t, err, ErrInvalidPEMBlock)
	privateKeyBytes := ExportPrivateKey(privateKey)
	_, err = ImportPrivateKey(privateKeyBytes)
	require.NoError(t, err)
	publicKey := &privateKey.PublicKey
	publicKeyBytes := ExportPublicKey(publicKey)
	_, err = ImportPublicKey(publicKeyBytes)
	require.NoError(t, err)
	// invalid public key
	_, err = ImportPublicKey(nil)
	require.Error(t, err)
	signature, err := Sign(privateKey, file)
	require.NoError(t, err)
	require.True(t, Verify(publicKey, file, signature), "invalid data")
	require.False(t, Verify(publicKey, file[:10], signature), "error verify")
	require.False(t, Verify(publicKey, file[:10], nil), "error verify")
	data := bytes.Repeat([]byte{0}, 128)
	cipherData, err := Encrypt(publicKey, data)
	require.NoError(t, err)
	plainData, err := Decrypt(privateKey, cipherData)
	require.NoError(t, err)
	require.Equal(t, plainData, data)
}

func BenchmarkSign(b *testing.B) {
	privateKey, err := GenerateKey(2048)
	require.NoError(b, err)
	data := bytes.Repeat([]byte{0}, 4096)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Sign(privateKey, data)
		// _, err := Sign(privateKey, data)
		// require.NoError(b, err)
	}
}

func BenchmarkVerify(b *testing.B) {
	privateKey, err := GenerateKey(2048)
	require.NoError(b, err)
	data := bytes.Repeat([]byte{0}, 256)
	signature, err := Sign(privateKey, data)
	require.NoError(b, err)
	publicKey := &privateKey.PublicKey
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Verify(publicKey, data, signature)
		// require.True(b, Verify(publicKey, data, signature), "verify failed")
	}
	b.StopTimer()
}

func BenchmarkSignVerify(b *testing.B) {
	privateKey, err := GenerateKey(2048)
	require.NoError(b, err)
	data := bytes.Repeat([]byte{0}, 256)
	publicKey := &privateKey.PublicKey
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		signature, _ := Sign(privateKey, data)
		Verify(publicKey, data, signature)
		// signature, err := Sign(privateKey, data)
		// require.NoError(b, err)
		// require.True(b, Verify(publicKey, data, signature), "verify failed")
	}
	b.StopTimer()
}
