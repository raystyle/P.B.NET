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

	require.EqualError(t, ank.Validate(), "invalid public key size")
	ank.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

	require.EqualError(t, ank.Validate(), "invalid key exchange public key size")
	ank.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)

	require.NoError(t, ank.Validate())
}

func TestQueryBeaconKey_SetID(t *testing.T) {
	qbk := new(QueryBeaconKey)
	g := testGenerateGUID()
	qbk.SetID(g)
	require.Equal(t, *g, qbk.ID)
}

func TestAnswerBeaconKey_Validate(t *testing.T) {
	abk := new(AnswerBeaconKey)

	require.EqualError(t, abk.Validate(), "invalid public key size")
	abk.PublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

	require.EqualError(t, abk.Validate(), "invalid key exchange public key size")
	abk.KexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)

	require.NoError(t, abk.Validate())
}

func TestUpdateNodeRequest_SetID(t *testing.T) {
	unr := new(UpdateNodeRequest)
	g := testGenerateGUID()
	unr.SetID(g)
	require.Equal(t, *g, unr.ID)
}
