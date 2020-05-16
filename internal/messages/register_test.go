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

	t.Run("invalid public key size", func(t *testing.T) {
		err := nrr.Validate()
		require.EqualError(t, err, "invalid public key size")
	})

	t.Run("invalid key exchange public key size", func(t *testing.T) {
		nrr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

		err := nrr.Validate()
		require.EqualError(t, err, "invalid key exchange public key size")
	})

	t.Run("empty system information", func(t *testing.T) {
		nrr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)
		nrr.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)

		err := nrr.Validate()
		require.EqualError(t, err, "empty system information")
	})

	t.Run("valid", func(t *testing.T) {
		nrr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)
		nrr.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)
		nrr.SystemInfo = new(info.System)

		err := nrr.Validate()
		require.NoError(t, err)
	})
}

func TestNodeRegisterResponse_Validate(t *testing.T) {
	nrr := new(NodeRegisterResponse)

	t.Run("invalid public key size", func(t *testing.T) {
		err := nrr.Validate()
		require.EqualError(t, err, "invalid public key size")

	})

	t.Run("invalid key exchange public key size", func(t *testing.T) {
		nrr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

		err := nrr.Validate()
		require.EqualError(t, err, "invalid key exchange public key size")
	})

	t.Run("unknown register result", func(t *testing.T) {
		nrr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)
		nrr.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)

		err := nrr.Validate()
		require.EqualError(t, err, "unknown register result")
	})

	t.Run("invalid certificate size", func(t *testing.T) {
		nrr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)
		nrr.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)
		nrr.Result = RegisterResultAccept

		err := nrr.Validate()
		require.EqualError(t, err, "invalid certificate size")
	})

	t.Run("valid", func(t *testing.T) {
		nrr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)
		nrr.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)
		nrr.Result = RegisterResultAccept
		nrr.Certificate = bytes.Repeat([]byte{0}, protocol.CertificateSize)

		err := nrr.Validate()
		require.NoError(t, err)
	})
}

func TestBeaconRegisterRequest_Validate(t *testing.T) {
	brr := new(BeaconRegisterRequest)

	t.Run("invalid public key size", func(t *testing.T) {
		err := brr.Validate()
		require.EqualError(t, err, "invalid public key size")
	})

	t.Run("invalid key exchange public key size", func(t *testing.T) {
		brr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

		err := brr.Validate()
		require.EqualError(t, err, "invalid key exchange public key size")
	})

	t.Run("empty system info", func(t *testing.T) {
		brr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)
		brr.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)

		err := brr.Validate()
		require.EqualError(t, err, "empty system info")
	})

	t.Run("valid", func(t *testing.T) {
		brr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)
		brr.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)
		brr.SystemInfo = new(info.System)

		err := brr.Validate()
		require.NoError(t, err)
	})
}

func TestBeaconRegisterResponse_Validate(t *testing.T) {
	nrr := new(BeaconRegisterResponse)

	t.Run("invalid public key size", func(t *testing.T) {
		err := nrr.Validate()
		require.EqualError(t, err, "invalid public key size")
	})

	t.Run("invalid key exchange public key size", func(t *testing.T) {
		nrr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

		err := nrr.Validate()
		require.EqualError(t, err, "invalid key exchange public key size")
	})

	t.Run("unknown register result", func(t *testing.T) {
		nrr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)
		nrr.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)

		err := nrr.Validate()
		require.EqualError(t, err, "unknown register result")
	})

	t.Run("valid", func(t *testing.T) {
		nrr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)
		nrr.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)
		nrr.Result = RegisterResultAccept

		err := nrr.Validate()
		require.NoError(t, err)
	})
}

func TestEncryptedRegisterRequest_SetID(t *testing.T) {
	ERR := new(EncryptedRegisterRequest)
	g := testGenerateGUID()
	ERR.SetID(g)
	require.Equal(t, *g, ERR.ID)
}

func TestEncryptedRegisterRequest_Validate(t *testing.T) {
	ERR := new(EncryptedRegisterRequest)

	t.Run("invalid key exchange public key size", func(t *testing.T) {
		err := ERR.Validate()
		require.EqualError(t, err, "invalid key exchange public key size")
	})

	t.Run("invalid encrypted request data size", func(t *testing.T) {
		ERR.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)

		err := ERR.Validate()
		require.EqualError(t, err, "invalid encrypted request data size")
	})

	t.Run("valid", func(t *testing.T) {
		ERR.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)
		ERR.EncRequest = bytes.Repeat([]byte{0}, aes.BlockSize)

		err := ERR.Validate()
		require.NoError(t, err)
	})
}
