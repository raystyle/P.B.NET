package messages

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/curve25519"
	"project/internal/crypto/ed25519"
)

func TestAnswerNodeKey_Validate(t *testing.T) {
	ank := new(AnswerNodeKey)

	require.EqualError(t, ank.Validate(), "invalid node public key size")
	ank.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

	require.EqualError(t, ank.Validate(), "invalid node key exchange public key size")
	ank.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)

	require.NoError(t, ank.Validate())
}

func TestAnswerBeaconKey_Validate(t *testing.T) {
	abk := new(AnswerBeaconKey)

	require.EqualError(t, abk.Validate(), "invalid beacon public key size")
	abk.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

	require.EqualError(t, abk.Validate(), "invalid beacon key exchange public key size")
	abk.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)

	require.NoError(t, abk.Validate())
}
