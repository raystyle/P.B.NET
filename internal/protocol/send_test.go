package protocol

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/guid"
	"project/internal/random"
)

func testGenerateSend(t *testing.T) *Send {
	rawS := new(Send)
	err := rawS.GUID.Write(bytes.Repeat([]byte{1}, guid.Size))
	require.NoError(t, err)
	err = rawS.RoleGUID.Write(bytes.Repeat([]byte{2}, guid.Size))
	require.NoError(t, err)
	rawS.Deflate = 1
	rawS.Hash = bytes.Repeat([]byte{3}, sha256.Size)
	return rawS
}

func TestSend_Unpack(t *testing.T) {
	t.Run("invalid send packet size", func(t *testing.T) {
		err := testGenerateSend(t).Unpack(nil)
		require.EqualError(t, err, "invalid send packet size")
	})

	rawData := new(bytes.Buffer)

	t.Run("smLen > mLen", func(t *testing.T) {
		rawS := testGenerateSend(t)
		rawS.Message = bytes.Repeat([]byte{4}, aes.BlockSize)
		rawS.Pack(rawData)

		newS := NewSend()
		err := newS.Unpack(rawData.Bytes())
		require.NoError(t, err)
		require.Equal(t, rawS, newS)
	})

	t.Run("smLen == mLen", func(t *testing.T) {
		rawS := testGenerateSend(t)
		rawS.Message = bytes.Repeat([]byte{4}, 2*aes.BlockSize)
		rawData.Reset()
		rawS.Pack(rawData)

		newS := NewSend()
		err := newS.Unpack(rawData.Bytes())
		require.NoError(t, err)
		require.Equal(t, rawS, newS)
	})

	t.Run("smLen < mLen", func(t *testing.T) {
		// minus smLen
		rawS := testGenerateSend(t)
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
		rawS := testGenerateSend(t)
		rawS.Message = bytes.Repeat([]byte{4}, 4*aes.BlockSize)
		rawData.Reset()
		rawS.Pack(rawData)

		newS := NewSend()
		err := newS.Unpack(rawData.Bytes())
		require.NoError(t, err)
		require.Equal(t, rawS, newS)
	})

	t.Run("fuzz", func(t *testing.T) {
		rawS := testGenerateSend(t)
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

	t.Run("invalid deflate flag", func(t *testing.T) {
		s.Deflate = 3

		err := s.Validate()
		require.EqualError(t, err, "invalid deflate flag")
	})

	t.Run("invalid hmac hash size", func(t *testing.T) {
		s.Deflate = 0

		err := s.Validate()
		require.EqualError(t, err, "invalid hmac hash size")
	})

	t.Run("invalid message size", func(t *testing.T) {
		s.Hash = bytes.Repeat([]byte{0}, sha256.Size)

		err := s.Validate()
		require.EqualError(t, err, "invalid message size")

		s.Message = bytes.Repeat([]byte{0}, 30)

		err = s.Validate()
		require.EqualError(t, err, "invalid message size")
	})

	t.Run("ok", func(t *testing.T) {
		s.Message = bytes.Repeat([]byte{0}, aes.BlockSize)

		err := s.Validate()
		require.NoError(t, err)
	})
}

func TestSendResult_Clean(t *testing.T) {
	new(SendResult).Clean()
}

func TestAcknowledge_Unpack(t *testing.T) {
	rawAck := new(Acknowledge)

	err := rawAck.GUID.Write(bytes.Repeat([]byte{1}, guid.Size))
	require.NoError(t, err)
	err = rawAck.RoleGUID.Write(bytes.Repeat([]byte{2}, guid.Size))
	require.NoError(t, err)
	err = rawAck.SendGUID.Write(bytes.Repeat([]byte{3}, guid.Size))
	require.NoError(t, err)
	rawAck.Hash = bytes.Repeat([]byte{4}, sha256.Size)
	rawData := new(bytes.Buffer)
	rawAck.Pack(rawData)

	newAck := NewAcknowledge()
	err = newAck.Unpack(nil)
	require.Error(t, err)
	err = newAck.Unpack(rawData.Bytes())
	require.NoError(t, err)
	require.Equal(t, rawAck, newAck)
}

func TestAcknowledge_Validate(t *testing.T) {
	ack := new(Acknowledge)

	err := ack.Validate()
	require.EqualError(t, err, "invalid hmac hash size")

	ack.Hash = bytes.Repeat([]byte{0}, sha256.Size)
	err = ack.Validate()
	require.NoError(t, err)
}

func TestAcknowledgeResult_Clean(t *testing.T) {
	new(AcknowledgeResult).Clean()
}

func TestQuery_Unpack(t *testing.T) {
	rawQuery := new(Query)
	err := rawQuery.GUID.Write(bytes.Repeat([]byte{1}, guid.Size))
	require.NoError(t, err)
	err = rawQuery.BeaconGUID.Write(bytes.Repeat([]byte{2}, guid.Size))
	require.NoError(t, err)
	rawQuery.Index = 10
	rawQuery.Hash = bytes.Repeat([]byte{3}, sha256.Size)
	rawData := new(bytes.Buffer)
	rawQuery.Pack(rawData)

	newQuery := NewQuery()
	err = newQuery.Unpack(nil)
	require.Error(t, err)
	err = newQuery.Unpack(rawData.Bytes())
	require.NoError(t, err)
	require.Equal(t, rawQuery, newQuery)
}

func TestQuery_Validate(t *testing.T) {
	q := new(Query)

	err := q.Validate()
	require.EqualError(t, err, "invalid hmac hash size")

	q.Hash = bytes.Repeat([]byte{0}, sha256.Size)

	err = q.Validate()
	require.NoError(t, err)
}

func TestQueryResult_Clean(t *testing.T) {
	new(QueryResult).Clean()
}

func testGenerateAnswer(t *testing.T) *Answer {
	rawA := new(Answer)
	err := rawA.GUID.Write(bytes.Repeat([]byte{1}, guid.Size))
	require.NoError(t, err)
	err = rawA.BeaconGUID.Write(bytes.Repeat([]byte{2}, guid.Size))
	require.NoError(t, err)
	rawA.Index = 10
	rawA.Deflate = 1
	rawA.Hash = bytes.Repeat([]byte{3}, sha256.Size)
	return rawA
}

func TestAnswer_Unpack(t *testing.T) {
	t.Run("invalid answer packet size", func(t *testing.T) {
		err := testGenerateAnswer(t).Unpack(nil)
		require.EqualError(t, err, "invalid answer packet size")
	})

	rawData := new(bytes.Buffer)

	t.Run("amLen > mLen", func(t *testing.T) {
		rawA := testGenerateAnswer(t)
		rawA.Message = bytes.Repeat([]byte{4}, aes.BlockSize)
		rawA.Pack(rawData)

		newA := NewAnswer()
		err := newA.Unpack(rawData.Bytes())
		require.NoError(t, err)
		require.Equal(t, rawA, newA)
	})

	t.Run("amLen == mLen", func(t *testing.T) {
		rawA := testGenerateAnswer(t)
		rawA.Message = bytes.Repeat([]byte{4}, 2*aes.BlockSize)
		rawData.Reset()
		rawA.Pack(rawData)

		newA := NewAnswer()
		err := newA.Unpack(rawData.Bytes())
		require.NoError(t, err)
		require.Equal(t, rawA, newA)
	})

	t.Run("amLen < mLen", func(t *testing.T) {
		// minus amLen
		rawA := testGenerateAnswer(t)
		rawA.Message = bytes.Repeat([]byte{4}, aes.BlockSize)
		rawData.Reset()
		rawA.Pack(rawData)

		newA := NewAnswer()
		err := newA.Unpack(rawData.Bytes())
		require.NoError(t, err)

		rawA.Message = bytes.Repeat([]byte{4}, 2*aes.BlockSize)
		rawData.Reset()
		rawA.Pack(rawData)
		err = newA.Unpack(rawData.Bytes())
		require.NoError(t, err)
	})

	t.Run("cap(a.Message) < mLen", func(t *testing.T) {
		rawA := testGenerateAnswer(t)
		rawA.Message = bytes.Repeat([]byte{4}, 4*aes.BlockSize)
		rawData.Reset()
		rawA.Pack(rawData)

		newA := NewAnswer()
		err := newA.Unpack(rawData.Bytes())
		require.NoError(t, err)
		require.Equal(t, rawA, newA)
	})

	t.Run("fuzz", func(t *testing.T) {
		rawA := testGenerateAnswer(t)
		newA := NewAnswer()
		for i := 0; i < 8192; i++ {
			size := 16 + random.Int(512)
			rawA.Message = bytes.Repeat([]byte{4}, size)
			rawData.Reset()
			rawA.Pack(rawData)

			err := newA.Unpack(rawData.Bytes())
			require.NoError(t, err)
			require.Equal(t, rawA, newA)
		}
	})
}

func TestAnswer_Validate(t *testing.T) {
	a := new(Answer)

	t.Run("invalid deflate flag", func(t *testing.T) {
		a.Deflate = 3

		err := a.Validate()
		require.EqualError(t, err, "invalid deflate flag")
	})

	t.Run("invalid hmac hash size", func(t *testing.T) {
		a.Deflate = 1

		err := a.Validate()
		require.EqualError(t, err, "invalid hmac hash size")
	})

	t.Run("invalid message size", func(t *testing.T) {
		a.Hash = bytes.Repeat([]byte{0}, sha256.Size)

		err := a.Validate()
		require.EqualError(t, err, "invalid message size")

		a.Message = bytes.Repeat([]byte{0}, 30)

		err = a.Validate()
		require.EqualError(t, err, "invalid message size")
	})

	t.Run("ok", func(t *testing.T) {
		a.Message = bytes.Repeat([]byte{0}, aes.BlockSize)

		err := a.Validate()
		require.NoError(t, err)
	})
}

func TestAnswerResult_Clean(t *testing.T) {
	new(AnswerResult).Clean()
}
