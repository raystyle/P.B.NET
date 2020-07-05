package xio

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

type notEqualWriter struct{}

func (notEqualWriter) Write([]byte) (int, error) {
	return 0, nil
}

func TestCopyBufferWithContext(t *testing.T) {
	testdata := bytes.Repeat([]byte("hello"), 100)
	ctx := context.Background()

	t.Run("common", func(t *testing.T) {
		readBuf := new(bytes.Buffer)
		writeBuf := new(bytes.Buffer)

		readBuf.Write(testdata)

		n, err := CopyWithContext(ctx, writeBuf, readBuf)
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
		n, err := CopyWithContext(ctx, writeBuf, lr)
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
		n, err := CopyWithContext(ctx, writeBuf, lr)
		require.NoError(t, err)
		require.Equal(t, int64(0), n)
	})

	t.Run("failed to read", func(t *testing.T) {
		reader := testsuite.NewMockConnWithReadError()
		writer := new(bytes.Buffer)

		n, err := CopyWithContext(ctx, writer, reader)
		require.Error(t, err)
		require.Equal(t, int64(0), n)
	})

	t.Run("failed to write", func(t *testing.T) {
		reader := new(bytes.Buffer)
		reader.Write([]byte{1, 2, 3})
		writer := testsuite.NewMockConnWithWriteError()

		n, err := CopyWithContext(ctx, writer, reader)
		require.Error(t, err)
		require.Equal(t, int64(0), n)
	})

	t.Run("written not equal", func(t *testing.T) {
		reader := new(bytes.Buffer)
		reader.Write([]byte{1, 2, 3})
		writer := new(notEqualWriter)

		n, err := CopyWithContext(ctx, writer, reader)
		require.Error(t, err)
		require.Equal(t, int64(0), n)
	})
}
