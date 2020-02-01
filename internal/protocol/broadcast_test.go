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

func testGenerateBroadcast() *Broadcast {
	rawB := new(Broadcast)
	copy(rawB.GUID[:], bytes.Repeat([]byte{1}, guid.Size))
	rawB.Hash = bytes.Repeat([]byte{2}, sha256.Size)
	rawB.Signature = bytes.Repeat([]byte{3}, ed25519.SignatureSize)
	return rawB
}

func TestBroadcast_Unpack(t *testing.T) {
	t.Run("invalid broadcast packet size", func(t *testing.T) {
		err := testGenerateBroadcast().Unpack(nil)
		require.Error(t, err)
	})

	rawData := new(bytes.Buffer)

	t.Run("bmLen > mLen", func(t *testing.T) {
		rawB := testGenerateBroadcast()
		rawB.Message = bytes.Repeat([]byte{4}, aes.BlockSize)
		rawB.Pack(rawData)

		newB := NewBroadcast()
		err := newB.Unpack(rawData.Bytes())
		require.NoError(t, err)
		require.Equal(t, rawB, newB)
	})

	t.Run("bmLen == mLen", func(t *testing.T) {
		rawB := testGenerateBroadcast()
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
		rawB := testGenerateBroadcast()
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
		rawB := testGenerateBroadcast()
		rawB.Message = bytes.Repeat([]byte{4}, 4*aes.BlockSize)
		rawData.Reset()
		rawB.Pack(rawData)

		newB := NewBroadcast()
		err := newB.Unpack(rawData.Bytes())
		require.NoError(t, err)
		require.Equal(t, rawB, newB)
	})

	t.Run("fuzz", func(t *testing.T) {
		rawB := testGenerateBroadcast()
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

	require.EqualError(t, b.Validate(), "invalid hash size")

	b.Hash = bytes.Repeat([]byte{0}, sha256.Size)
	require.EqualError(t, b.Validate(), "invalid signature size")

	b.Signature = bytes.Repeat([]byte{0}, ed25519.SignatureSize)
	require.EqualError(t, b.Validate(), "invalid message size")
	b.Message = bytes.Repeat([]byte{0}, 30)
	require.EqualError(t, b.Validate(), "invalid message size")

	b.Message = bytes.Repeat([]byte{0}, aes.BlockSize)
	require.NoError(t, b.Validate())
}

func TestBroadcastResult_Clean(t *testing.T) {
	new(BroadcastResult).Clean()
}
