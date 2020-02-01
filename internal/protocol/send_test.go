package protocol

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/guid"
	"project/internal/random"
)

func testGenerateSend() *Send {
	rawS := new(Send)
	rawS.GUID = bytes.Repeat([]byte{1}, guid.Size)
	rawS.RoleGUID = bytes.Repeat([]byte{2}, guid.Size)
	rawS.Hash = bytes.Repeat([]byte{3}, sha256.Size)
	rawS.Signature = bytes.Repeat([]byte{4}, ed25519.SignatureSize)
	return rawS
}

func TestSend_Unpack(t *testing.T) {
	t.Run("invalid send packet size", func(t *testing.T) {
		err := testGenerateSend().Unpack(nil)
		require.Error(t, err)
	})

	rawData := new(bytes.Buffer)

	t.Run("smLen > mLen", func(t *testing.T) {
		rawS := testGenerateSend()
		rawS.Message = bytes.Repeat([]byte{4}, aes.BlockSize)
		rawS.Pack(rawData)

		newS := NewSend()
		err := newS.Unpack(rawData.Bytes())
		require.NoError(t, err)
		require.Equal(t, rawS, newS)
	})

	t.Run("smLen == mLen", func(t *testing.T) {
		rawS := testGenerateSend()
		rawS.Message = bytes.Repeat([]byte{4}, 2*aes.BlockSize)
		rawData.Reset()
		rawS.Pack(rawData)

		newS := NewSend()
		err := newS.Unpack(rawData.Bytes())
		require.NoError(t, err)
		require.Equal(t, rawS, newS)
	})

	t.Run("smLen < mLen", func(t *testing.T) {
		// minus bmLen
		rawS := testGenerateSend()
		rawS.Message = bytes.Repeat([]byte{4}, aes.BlockSize)
		rawData.Reset()
		rawS.Pack(rawData)

		newS := NewSend()
		err := newS.Unpack(rawData.Bytes())
		require.NoError(t, err)

		rawS.Message = bytes.Repeat([]byte{4}, 2*aes.BlockSize)
		rawData.Reset()
		rawS.Pack(rawData)
		err = newS.Unpack(rawData.Bytes())
		require.NoError(t, err)
	})

	t.Run("cap(s.Message) < mLen", func(t *testing.T) {
		rawS := testGenerateSend()
		rawS.Message = bytes.Repeat([]byte{4}, 4*aes.BlockSize)
		rawData.Reset()
		rawS.Pack(rawData)

		newS := NewSend()
		err := newS.Unpack(rawData.Bytes())
		require.NoError(t, err)
		require.Equal(t, rawS, newS)
	})

	t.Run("fuzz", func(t *testing.T) {
		rawS := testGenerateSend()
		newS := NewSend()
		for i := 0; i < 8192; i++ {
			size := 16 + random.Int(512)
			rawS.Message = bytes.Repeat([]byte{4}, size)
			rawData.Reset()
			rawS.Pack(rawData)

			err := newS.Unpack(rawData.Bytes())
			require.NoError(t, err)
			require.Equal(t, rawS, newS)
		}
	})
}

func TestSend_Validate(t *testing.T) {
	s := new(Send)
	require.EqualError(t, s.Validate(), "invalid guid size")

	s.GUID = CtrlGUID
	require.EqualError(t, s.Validate(), "invalid role guid size")

	s.RoleGUID = CtrlGUID
	require.EqualError(t, s.Validate(), "invalid hash size")

	s.Hash = bytes.Repeat([]byte{0}, sha256.Size)
	require.EqualError(t, s.Validate(), "invalid signature size")

	s.Signature = bytes.Repeat([]byte{0}, ed25519.SignatureSize)
	require.EqualError(t, s.Validate(), "invalid message size")
	s.Message = bytes.Repeat([]byte{0}, 30)
	require.EqualError(t, s.Validate(), "invalid message size")

	s.Message = bytes.Repeat([]byte{0}, aes.BlockSize)
	require.NoError(t, s.Validate())
}

func TestSendResult_Clean(t *testing.T) {
	new(SendResult).Clean()
}

func TestAcknowledge_Unpack(t *testing.T) {
	rawAck := new(Acknowledge)
	rawAck.GUID = bytes.Repeat([]byte{1}, guid.Size)
	rawAck.RoleGUID = bytes.Repeat([]byte{2}, guid.Size)
	rawAck.SendGUID = bytes.Repeat([]byte{3}, guid.Size)
	rawAck.Signature = bytes.Repeat([]byte{4}, ed25519.SignatureSize)
	rawData := new(bytes.Buffer)
	rawAck.Pack(rawData)

	newAck := NewAcknowledge()
	err := newAck.Unpack(nil)
	require.Error(t, err)
	err = newAck.Unpack(rawData.Bytes())
	require.NoError(t, err)
	require.Equal(t, rawAck, newAck)
}

func TestAcknowledge_Validate(t *testing.T) {
	ack := new(Acknowledge)
	require.EqualError(t, ack.Validate(), "invalid guid size")

	ack.GUID = CtrlGUID
	require.EqualError(t, ack.Validate(), "invalid role guid size")

	ack.RoleGUID = CtrlGUID
	require.EqualError(t, ack.Validate(), "invalid send guid size")

	ack.SendGUID = CtrlGUID
	require.EqualError(t, ack.Validate(), "invalid signature size")

	ack.Signature = bytes.Repeat([]byte{0}, ed25519.SignatureSize)
	require.NoError(t, ack.Validate())
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
