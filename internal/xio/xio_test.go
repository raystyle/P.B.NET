package xio

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCopyBufferWithContext(t *testing.T) {
	testdata := bytes.Repeat([]byte("hello"), 100)

	t.Run("common", func(t *testing.T) {
		readBuf := new(bytes.Buffer)
		writeBuf := new(bytes.Buffer)

		readBuf.Write(testdata)

		n, err := CopyWithContext(context.Background(), writeBuf, readBuf)
		require.NoError(t, err)
		require.Equal(t, int64(len(testdata)), n)

		require.Equal(t, testdata, writeBuf.Bytes())
	})

	t.Run("cancel", func(t *testing.T) {
		readBuf := new(bytes.Buffer)
		writeBuf := new(bytes.Buffer)

		readBuf.Write(testdata)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		n, err := CopyWithContext(ctx, writeBuf, readBuf)
		require.Equal(t, context.Canceled, err)
		require.Equal(t, int64(0), n)
	})

	t.Run("LimitedReader", func(t *testing.T) {
		readBuf := new(bytes.Buffer)
		writeBuf := new(bytes.Buffer)

		readBuf.Write(testdata)

		lr := io.LimitReader(readBuf, int64(len(testdata)))
		n, err := CopyWithContext(context.Background(), writeBuf, lr)
		require.NoError(t, err)
		require.Equal(t, int64(len(testdata)), n)

		require.Equal(t, testdata, writeBuf.Bytes())
	})

	t.Run("LimitedReader Negative", func(t *testing.T) {
		readBuf := new(bytes.Buffer)
		writeBuf := new(bytes.Buffer)

		readBuf.Write(testdata)

		lr := &io.LimitedReader{
			R: readBuf,
			N: -1,
		}
		n, err := CopyWithContext(context.Background(), writeBuf, lr)
		require.NoError(t, err)
		require.Equal(t, int64(0), n)
	})

	t.Run("failed to read", func(t *testing.T) {

	})

	t.Run("failed to write", func(t *testing.T) {

	})

	t.Run("written not equal", func(t *testing.T) {

	})
}
