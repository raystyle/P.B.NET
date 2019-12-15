package protocol

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
)

func TestBroadcast_Validate(t *testing.T) {
	b := new(Broadcast)
	require.EqualError(t, b.Validate(), "invalid guid size")

	b.GUID = CtrlGUID

	require.EqualError(t, b.Validate(), "invalid message size")
	b.Message = bytes.Repeat([]byte{0}, 30)
	require.EqualError(t, b.Validate(), "invalid message size")

	b.Message = bytes.Repeat([]byte{0}, aes.BlockSize)

	require.EqualError(t, b.Validate(), "invalid hash size")

	b.Hash = bytes.Repeat([]byte{0}, sha256.Size)
	require.EqualError(t, b.Validate(), "invalid signature size")

	b.Signature = bytes.Repeat([]byte{0}, ed25519.SignatureSize)
	require.NoError(t, b.Validate())
}

func TestBroadcastResult_Clean(t *testing.T) {
	new(BroadcastResult).Clean()
}
