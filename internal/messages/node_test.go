package messages

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/curve25519"
	"project/internal/crypto/ed25519"
)

func TestQueryNodeKey_SetID(t *testing.T) {
	qnk := new(QueryNodeKey)
	g := testGenerateGUID()
	qnk.SetID(g)
	require.Equal(t, *g, qnk.ID)
}

func TestAnswerNodeKey_Validate(t *testing.T) {
	ank := new(AnswerNodeKey)

	t.Run("invalid public key size", func(t *testing.T) {
		err := ank.Validate()
		require.EqualError(t, err, "invalid public key size")
	})

	t.Run("invalid key exchange public key size", func(t *testing.T) {
		ank.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

		err := ank.Validate()
		require.EqualError(t, err, "invalid key exchange public key size")
	})

	t.Run("valid", func(t *testing.T) {
		ank.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)
		ank.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)

		err := ank.Validate()
		require.NoError(t, err)
	})
}

func TestQueryBeaconKey_SetID(t *testing.T) {
	qbk := new(QueryBeaconKey)
	g := testGenerateGUID()
	qbk.SetID(g)
	require.Equal(t, *g, qbk.ID)
}

func TestAnswerBeaconKey_Validate(t *testing.T) {
	abk := new(AnswerBeaconKey)

	t.Run("invalid public key size", func(t *testing.T) {
		err := abk.Validate()
		require.EqualError(t, err, "invalid public key size")
	})

	t.Run("invalid key exchange public key size", func(t *testing.T) {
		abk.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

		err := abk.Validate()
		require.EqualError(t, err, "invalid key exchange public key size")
	})

	t.Run("valid", func(t *testing.T) {
		abk.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)
		abk.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)

		err := abk.Validate()
		require.NoError(t, err)
	})
}

func TestUpdateNodeRequest_SetID(t *testing.T) {
	unr := new(UpdateNodeRequest)
	g := testGenerateGUID()
	unr.SetID(g)
	require.Equal(t, *g, unr.ID)
}
