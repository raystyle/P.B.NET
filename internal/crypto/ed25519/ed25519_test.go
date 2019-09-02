package ed25519

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ed25519(t *testing.T) {
	pri, err := Generate_Key()
	require.NoError(t, err)
	message := []byte("test message")
	signature := Sign(pri, message)
	require.True(t, len(signature) == Signature_Size)
	require.True(t, Verify(pri.PublicKey(), message, signature))
	pri, err = Import_PrivateKey(bytes.Repeat([]byte{0, 1}, 32))
	require.NoError(t, err)
	require.NotNil(t, pri)
	pub, err := Import_PublicKey(bytes.Repeat([]byte{0, 1}, 16))
	require.NoError(t, err)
	require.NotNil(t, pub)
	pri, err = Import_PrivateKey(bytes.Repeat([]byte{0, 1}, 161))
	require.Equal(t, ERR_INVALID_PRIVATEKEY, err)
	require.Nil(t, pri)
	pub, err = Import_PublicKey(bytes.Repeat([]byte{0, 1}, 161))
	require.Equal(t, ERR_INVALID_PUBLICKEY, err)
	require.Nil(t, pub)
}

func Benchmark_ed25519_sign(b *testing.B) {
	pri, err := Generate_Key()
	require.Nil(b, err, err)
	msg := bytes.Repeat([]byte{0}, 256)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Sign(pri, msg)
	}
	b.StopTimer()
}

func Benchmark_ed25519_verify(b *testing.B) {
	pri, err := Generate_Key()
	require.Nil(b, err, err)
	msg := bytes.Repeat([]byte{0}, 256)
	signature := Sign(pri, msg)
	pub := pri.PublicKey()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Verify(pub, msg, signature)
	}
	b.StopTimer()
}
