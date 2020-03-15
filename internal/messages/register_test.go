package messages

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/crypto/ed25519"
	"project/internal/module/info"
	"project/internal/protocol"
)

func TestNodeRegisterRequest_Validate(t *testing.T) {
	nrr := new(NodeRegisterRequest)

	require.EqualError(t, nrr.Validate(), "invalid public key size")
	nrr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

	require.EqualError(t, nrr.Validate(), "invalid key exchange public key size")
	nrr.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)

	require.EqualError(t, nrr.Validate(), "empty system info")
	nrr.SystemInfo = new(info.System)

	require.NoError(t, nrr.Validate())
}

func TestNodeRegisterResponse_Validate(t *testing.T) {
	nrr := new(NodeRegisterResponse)

	require.EqualError(t, nrr.Validate(), "invalid public key size")
	nrr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

	require.EqualError(t, nrr.Validate(), "invalid key exchange public key size")
	nrr.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)

	require.EqualError(t, nrr.Validate(), "unknown register result")
	nrr.Result = RegisterResultAccept

	require.EqualError(t, nrr.Validate(), "invalid certificate size")
	nrr.Certificate = bytes.Repeat([]byte{0}, protocol.CertificateSize)

	require.NoError(t, nrr.Validate())
}

func TestBeaconRegisterRequest_Validate(t *testing.T) {
	brr := new(BeaconRegisterRequest)

	require.EqualError(t, brr.Validate(), "invalid public key size")
	brr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

	require.EqualError(t, brr.Validate(), "invalid key exchange public key size")
	brr.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)

	require.EqualError(t, brr.Validate(), "empty system info")
	brr.SystemInfo = new(info.System)

	require.NoError(t, brr.Validate())
}

func TestBeaconRegisterResponse_Validate(t *testing.T) {
	nrr := new(BeaconRegisterResponse)

	require.EqualError(t, nrr.Validate(), "invalid public key size")
	nrr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

	require.EqualError(t, nrr.Validate(), "invalid key exchange public key size")
	nrr.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)

	require.EqualError(t, nrr.Validate(), "unknown register result")
	nrr.Result = RegisterResultAccept

	require.NoError(t, nrr.Validate())
}

func TestEncryptedRegisterRequest(t *testing.T) {
	t.Run("SetID", func(t *testing.T) {
		ERR := new(EncryptedRegisterRequest)
		g := testGenerateGUID()
		ERR.SetID(g)
		require.Equal(t, *g, ERR.ID)
	})

	t.Run("Validate", func(t *testing.T) {
		ERR := new(EncryptedRegisterRequest)

		require.EqualError(t, ERR.Validate(), "invalid key exchange public key size")
		ERR.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)

		require.EqualError(t, ERR.Validate(), "invalid encrypted request data size")
		ERR.EncRequest = bytes.Repeat([]byte{0}, aes.BlockSize)

		require.NoError(t, ERR.Validate())
	})
}
