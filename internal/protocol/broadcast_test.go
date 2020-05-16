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

func testGenerateBroadcast(t *testing.T) *Broadcast {
	rawB := new(Broadcast)
	err := rawB.GUID.Write(bytes.Repeat([]byte{1}, guid.Size))
	require.NoError(t, err)
	rawB.Hash = bytes.Repeat([]byte{2}, sha256.Size)
	rawB.Deflate = 1
	rawB.Signature = bytes.Repeat([]byte{3}, ed25519.SignatureSize)
	return rawB
}

func TestBroadcast_Unpack(t *testing.T) {
	t.Run("invalid broadcast packet size", func(t *testing.T) {
		err := testGenerateBroadcast(t).Unpack(nil)
		require.EqualError(t, err, "invalid broadcast packet size")
	})

	rawData := new(bytes.Buffer)

	t.Run("bmLen > mLen", func(t *testing.T) {
		rawB := testGenerateBroadcast(t)
		rawB.Message = bytes.Repeat([]byte{4}, aes.BlockSize)
		rawB.Pack(rawData)

		newB := NewBroadcast()
		err := newB.Unpack(rawData.Bytes())
		require.NoError(t, err)
		require.Equal(t, rawB, newB)
	})

	t.Run("bmLen == mLen", func(t *testing.T) {
		rawB := testGenerateBroadcast(t)
		rawB.Message = bytes.Repeat([]byte{4}, 2*aes.BlockSize)
		rawData.Reset()
		rawB.Pack(rawData)

		newB := NewBroadcast()
		err := newB.Unpack(rawData.Bytes())
		require.NoError(t, err)
		require.Equal(t, rawB, newB)
	})

	t.Run("bmLen < mLen", func(t *testing.T) {
		// minus bmLen
		rawB := testGenerateBroadcast(t)
		rawB.Message = bytes.Repeat([]byte{4}, aes.BlockSize)
		rawData.Reset()
		rawB.Pack(rawData)

		newB := NewBroadcast()
		err := newB.Unpack(rawData.Bytes())
		require.NoError(t, err)

		rawB.Message = bytes.Repeat([]byte{4}, 2*aes.BlockSize)
		rawData.Reset()
		rawB.Pack(rawData)
		err = newB.Unpack(rawData.Bytes())
		require.NoError(t, err)
	})

	t.Run("cap(b.Message) < mLen", func(t *testing.T) {
		rawB := testGenerateBroadcast(t)
		rawB.Message = bytes.Repeat([]byte{4}, 4*aes.BlockSize)
		rawData.Reset()
		rawB.Pack(rawData)

		newB := NewBroadcast()
		err := newB.Unpack(rawData.Bytes())
		require.NoError(t, err)
		require.Equal(t, rawB, newB)
	})

	t.Run("fuzz", func(t *testing.T) {
		rawB := testGenerateBroadcast(t)
		newB := NewBroadcast()
		for i := 0; i < 8192; i++ {
			size := 16 + random.Int(512)
			rawB.Message = bytes.Repeat([]byte{4}, size)
			rawData.Reset()
			rawB.Pack(rawData)

			err := newB.Unpack(rawData.Bytes())
			require.NoError(t, err)
			require.Equal(t, rawB, newB)
		}
	})
}

func TestBroadcast_Validate(t *testing.T) {
	b := new(Broadcast)

	t.Run("invalid hash size", func(t *testing.T) {
		err := b.Validate()
		require.EqualError(t, err, "invalid hash size")
	})

	t.Run("invalid signature size", func(t *testing.T) {
		b.Hash = bytes.Repeat([]byte{0}, sha256.Size)

		err := b.Validate()
		require.EqualError(t, err, "invalid signature size")
	})

	t.Run("invalid deflate flag", func(t *testing.T) {
		b.Signature = bytes.Repeat([]byte{0}, ed25519.SignatureSize)
		b.Deflate = 3

		err := b.Validate()
		require.EqualError(t, err, "invalid deflate flag")
	})

	t.Run("invalid message size", func(t *testing.T) {
		b.Deflate = 1

		err := b.Validate()
		require.EqualError(t, err, "invalid message size")

		b.Message = bytes.Repeat([]byte{0}, 30)

		err = b.Validate()
		require.EqualError(t, err, "invalid message size")
	})

	t.Run("ok", func(t *testing.T) {
		b.Message = bytes.Repeat([]byte{0}, aes.BlockSize)
		err := b.Validate()
		require.NoError(t, err)
	})
}

func TestBroadcastResult_Clean(t *testing.T) {
	new(BroadcastResult).Clean()
}
