package ecdsa

import (
	"bytes"
	"crypto/elliptic"
	"crypto/x509"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/rsa"
)

func Test_ECDSA(t *testing.T) {
	// generate key
	privatekey, err := Generate_Key(elliptic.P256())
	require.Nil(t, err, err)
	// import private key pem
	file, err := ioutil.ReadFile("ecdsa.key")
	require.Nil(t, err, err)
	// import private key ec
	_, err = Import_PrivateKey_PEM(file)
	require.Nil(t, err, err)
	// export private key
	_, err = Export_PrivateKey(privatekey)
	require.Nil(t, err, err)
	// import private key pkcs8
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(privatekey)
	_, err = Import_PrivateKey(pkcs8)
	require.Nil(t, err, err)
	// export & import public key
	publickey := &privatekey.PublicKey
	publickey_bytes := Export_PublicKey(publickey)
	_, err = Import_PublicKey(publickey_bytes)
	require.Nil(t, err, err)
	// sign & verify
	signature, err := Sign(privatekey, file)
	require.Nil(t, err, err)
	require.True(t, Verify(publickey, file, signature), "invalid data")
	require.False(t, Verify(publickey, file[:10], signature), "error verify")
	require.False(t, Verify(publickey, file[:10], nil), "error verify")
	// import private key pem error
	_, err = Import_PrivateKey_PEM(nil)
	require.Equal(t, err, ERR_INVALID_PEM_BLOCK, err)
	// invalid public key
	_, err = Import_PublicKey(nil)
	require.NotNil(t, err)
	// rsa public key
	rsa_pri, _ := rsa.Generate_Key(1024)
	rsa_pub_b, _ := x509.MarshalPKIXPublicKey(&rsa_pri.PublicKey)
	_, err = Import_PublicKey(rsa_pub_b)
	require.Equal(t, ERR_NOT_PUBLIC_KEY, err, err)
	// import rsa private key
	rsa_pri_b, _ := x509.MarshalPKCS8PrivateKey(rsa_pri)
	_, err = Import_PrivateKey(rsa_pri_b)
	require.Equal(t, ERR_NOT_PRIVATE_KEY, err, err)
	// error sign
	privatekey.PublicKey.Curve.Params().N.SetBytes(nil)
	_, err = Sign(privatekey, file)
	require.NotNil(t, err)
	// error verify
	msg := "error verify"
	require.False(t, Verify(publickey, file, []byte{0}), msg)
	require.False(t, Verify(publickey, file, []byte{0, 3, 22, 22}), msg)
	require.False(t, Verify(publickey, file, []byte{0, 2, 22, 22}), msg)
	require.False(t, Verify(publickey, file, []byte{0, 2, 22, 22, 0, 1}), msg)
}

func Benchmark_Sign(b *testing.B) {
	privatekey, err := Generate_Key(elliptic.P256())
	require.Nil(b, err, err)
	data := bytes.Repeat([]byte{0}, 256)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Sign(privatekey, data)
	}
	b.StopTimer()
}

func Benchmark_Verify(b *testing.B) {
	privatekey, err := Generate_Key(elliptic.P256())
	require.Nil(b, err, err)
	data := bytes.Repeat([]byte{0}, 256)
	signature, err := Sign(privatekey, data)
	require.Nil(b, err, err)
	publickey := &privatekey.PublicKey
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Verify(publickey, data, signature)
		//require.True(b, Verify(publickey, data, signature), "verify failed")
	}
	b.StopTimer()
}

func Benchmark_Sign_Verify(b *testing.B) {
	privatekey, err := Generate_Key(elliptic.P256())
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
