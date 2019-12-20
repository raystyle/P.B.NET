package messages

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/ed25519"
	"project/internal/modules/info"
	"project/internal/protocol"
)

func TestNodeRegisterRequest_Validate(t *testing.T) {
	nrr := new(NodeRegisterRequest)
	require.EqualError(t, nrr.Validate(), "invalid guid size")

	nrr.GUID = protocol.CtrlGUID

	require.EqualError(t, nrr.Validate(), "invalid public key size")
	nrr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

	require.EqualError(t, nrr.Validate(), "invalid key exchange public key size")
	nrr.KexPublicKey = bytes.Repeat([]byte{0}, 32)

	require.EqualError(t, nrr.Validate(), "empty system info")
	nrr.SystemInfo = new(info.System)

	require.NoError(t, nrr.Validate())
}

func TestNodeRegisterResponse_Validate(t *testing.T) {
	nrr := new(NodeRegisterResponse)
	require.EqualError(t, nrr.Validate(), "invalid guid size")

	nrr.GUID = protocol.CtrlGUID

	require.EqualError(t, nrr.Validate(), "invalid public key size")
	nrr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

	require.EqualError(t, nrr.Validate(), "invalid key exchange public key size")
	nrr.KexPublicKey = bytes.Repeat([]byte{0}, 32)

	require.EqualError(t, nrr.Validate(), "unknown node register result")
	nrr.Result = RegisterResultAccept

	require.EqualError(t, nrr.Validate(), "invalid certificate size")
	nrr.Certificates = bytes.Repeat([]byte{0}, 2*ed25519.SignatureSize)

	require.NoError(t, nrr.Validate())
}

func TestBeaconRegisterRequest_Validate(t *testing.T) {
	brr := new(BeaconRegisterRequest)
	require.EqualError(t, brr.Validate(), "invalid guid size")

	brr.GUID = protocol.CtrlGUID

	require.EqualError(t, brr.Validate(), "invalid public key size")
	brr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

	require.EqualError(t, brr.Validate(), "invalid key exchange public key size")
	brr.KexPublicKey = bytes.Repeat([]byte{0}, 32)

	require.EqualError(t, brr.Validate(), "empty system info")
	brr.SystemInfo = new(info.System)

	require.NoError(t, brr.Validate())
}

func TestBeaconRegisterResponse_Validate(t *testing.T) {
	nrr := new(BeaconRegisterResponse)
	require.EqualError(t, nrr.Validate(), "invalid guid size")

	nrr.GUID = protocol.CtrlGUID

	require.EqualError(t, nrr.Validate(), "invalid public key size")
	nrr.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

	require.EqualError(t, nrr.Validate(), "invalid key exchange public key size")
	nrr.KexPublicKey = bytes.Repeat([]byte{0}, 32)

	require.EqualError(t, nrr.Validate(), "unknown beacon register result")
	nrr.Result = RegisterResultAccept

	require.NoError(t, nrr.Validate())
}
