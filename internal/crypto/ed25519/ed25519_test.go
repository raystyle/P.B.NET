package ed25519

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
)

func TestED25519(t *testing.T) {
	pri, err := GenerateKey()
	require.NoError(t, err)

	t.Run("sign and verify", func(t *testing.T) {
		message := []byte("test message")
		signature := Sign(pri, message)
		require.Len(t, signature, SignatureSize)
		require.True(t, Verify(pri.PublicKey(), message, signature))
	})

	t.Run("import private and public key", func(t *testing.T) {
		pri, err = ImportPrivateKey(bytes.Repeat([]byte{0, 1}, 32))
		require.NoError(t, err)
		require.NotNil(t, pri)

		pub, err := ImportPublicKey(bytes.Repeat([]byte{0, 1}, 16))
		require.NoError(t, err)
		require.NotNil(t, pub)

		pri, err = ImportPrivateKey(bytes.Repeat([]byte{0, 1}, 161))
		require.Equal(t, ErrInvalidPrivateKey, err)
		require.Nil(t, pri)

		pub, err = ImportPublicKey(bytes.Repeat([]byte{0, 1}, 161))
		require.Equal(t, ErrInvalidPublicKey, err)
		require.Nil(t, pub)
	})

	t.Run("failed to generate key", func(t *testing.T) {
		patch := func(io.Reader, []byte) (int, error) {
			return 0, monkey.Error
		}
		pg := monkey.Patch(io.ReadFull, patch)
		defer pg.Unpatch()

		_, err := GenerateKey()
		monkey.IsMonkeyError(t, err)
	})
}

func BenchmarkSign(b *testing.B) {
	pri, err := GenerateKey()
	require.NoError(b, err)
	msg := bytes.Repeat([]byte{0}, 256)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		Sign(pri, msg)
	}

	b.StopTimer()
}

func BenchmarkVerify(b *testing.B) {
	pri, err := GenerateKey()
	require.NoError(b, err)
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
