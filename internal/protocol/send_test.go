package protocol

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/crypto/sha256"
)

func TestSend_Validate(t *testing.T) {
	s := new(Send)
	require.EqualError(t, s.Validate(), "invalid guid size")

	s.GUID = CtrlGUID

	require.EqualError(t, s.Validate(), "invalid role guid size")

	s.RoleGUID = CtrlGUID

	require.EqualError(t, s.Validate(), "invalid message size")
	s.Message = bytes.Repeat([]byte{0}, 30)
	require.EqualError(t, s.Validate(), "invalid message size")

	s.Message = bytes.Repeat([]byte{0}, aes.BlockSize)

	require.EqualError(t, s.Validate(), "invalid hash size")

	s.Hash = bytes.Repeat([]byte{0}, sha256.Size)
	require.EqualError(t, s.Validate(), "invalid signature size")

	s.Signature = bytes.Repeat([]byte{0}, ed25519.SignatureSize)
	require.NoError(t, s.Validate())
}

func TestSendResult_Clean(t *testing.T) {
	new(SendResult).Clean()
}

func TestAcknowledge_Validate(t *testing.T) {
	a := new(Acknowledge)
	require.EqualError(t, a.Validate(), "invalid guid size")

	a.GUID = CtrlGUID

	require.EqualError(t, a.Validate(), "invalid role guid size")

	a.RoleGUID = CtrlGUID

	require.EqualError(t, a.Validate(), "invalid send guid size")

	a.SendGUID = CtrlGUID

	require.EqualError(t, a.Validate(), "invalid signature size")

	a.Signature = bytes.Repeat([]byte{0}, ed25519.SignatureSize)
	require.NoError(t, a.Validate())
}

func TestAcknowledgeResult_Clean(t *testing.T) {
	new(AcknowledgeResult).Clean()
}

func TestQuery_Validate(t *testing.T) {
	q := new(Query)
	require.EqualError(t, q.Validate(), "invalid guid size")

	q.GUID = CtrlGUID

	require.EqualError(t, q.Validate(), "invalid beacon guid size")

	q.BeaconGUID = CtrlGUID

	require.EqualError(t, q.Validate(), "invalid signature size")

	q.Signature = bytes.Repeat([]byte{0}, ed25519.SignatureSize)
	require.NoError(t, q.Validate())
}

func TestQueryResult_Clean(t *testing.T) {
	new(QueryResult).Clean()
}

func TestAnswer_Validate(t *testing.T) {
	a := new(Answer)
	require.EqualError(t, a.Validate(), "invalid guid size")

	a.GUID = CtrlGUID

	require.EqualError(t, a.Validate(), "invalid beacon guid size")

	a.BeaconGUID = CtrlGUID

	require.EqualError(t, a.Validate(), "invalid message size")
	a.Message = bytes.Repeat([]byte{0}, 30)
	require.EqualError(t, a.Validate(), "invalid message size")

	a.Message = bytes.Repeat([]byte{0}, aes.BlockSize)

	require.EqualError(t, a.Validate(), "invalid hash size")

	a.Hash = bytes.Repeat([]byte{0}, sha256.Size)
	require.EqualError(t, a.Validate(), "invalid signature size")

	a.Signature = bytes.Repeat([]byte{0}, ed25519.SignatureSize)
	require.NoError(t, a.Validate())
}

func TestAnswerResult_Clean(t *testing.T) {
	new(AnswerResult).Clean()
}
