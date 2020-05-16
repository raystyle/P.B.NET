package virtualconn

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/guid"
	"project/internal/testsuite"
)

type pipe struct {
	sender   chan<- []byte
	receiver <-chan []byte
}

func (pipe *pipe) Send(ctx context.Context, data []byte) error {
	select {
	case pipe.sender <- data:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (pipe *pipe) Receive(ctx context.Context) ([]byte, error) {
	select {
	case data := <-pipe.receiver:
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func newPipe() (*pipe, *pipe) {
	ch1 := make(chan []byte)
	ch2 := make(chan []byte)
	p1 := pipe{
		sender:   ch1,
		receiver: ch2,
	}
	p2 := pipe{
		sender:   ch2,
		receiver: ch1,
	}
	return &p1, &p2
}

func TestPipe(t *testing.T) {
	pipe1, pipe2 := newPipe()

	// pipe1 send
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err := pipe1.Send(ctx, testsuite.Bytes())
		require.NoError(t, err)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	data, err := pipe2.Receive(ctx)
	require.NoError(t, err)

	require.Equal(t, testsuite.Bytes(), data)

	// pipe2 send
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err := pipe2.Send(ctx, testsuite.Bytes())
		require.NoError(t, err)
	}()

	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	data, err = pipe1.Receive(ctx)
	require.NoError(t, err)

	require.Equal(t, testsuite.Bytes(), data)
}

func testGenerateConnPair(t *testing.T) (*Conn, *Conn) {
	cGUID := new(guid.GUID)
	cPort := uint32(1)
	sGUID := new(guid.GUID)
	sPort := uint32(2)
	err := cGUID.Write(bytes.Repeat([]byte{1}, guid.Size))
	require.NoError(t, err)
	err = sGUID.Write(bytes.Repeat([]byte{2}, guid.Size))
	require.NoError(t, err)

	pipe1, pipe2 := newPipe()
	client := NewConn(pipe1, pipe1, cGUID, cPort, sGUID, sPort)
	server := NewConn(pipe2, pipe2, sGUID, sPort, cGUID, cPort)
	return server, client
}

func TestConn(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, client := testGenerateConnPair(t)
	testsuite.ConnCS(t, client, server, true)

	server, client = testGenerateConnPair(t)
	testsuite.ConnSC(t, server, client, true)
}

func TestConn_Deadline(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, client := testGenerateConnPair(t)
	err := server.Close()
	require.NoError(t, err)
	err = client.Close()
	require.NoError(t, err)

	do := func(conn *Conn) {
		all := func() {
			err := conn.SetDeadline(time.Now().Add(time.Second))
			require.Error(t, err)
		}
		read := func() {
			err := conn.SetReadDeadline(time.Now().Add(time.Second))
			require.Error(t, err)
		}
		write := func() {
			err := conn.SetWriteDeadline(time.Now().Add(time.Second))
			require.Error(t, err)
		}
		testsuite.RunParallel(all, read, write)
	}
	do(server)
	do(client)

	testsuite.IsDestroyed(t, server)
	testsuite.IsDestroyed(t, client)
}

func TestConn_WithBigData(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	testdata := bytes.Repeat(testsuite.Bytes(), 204800) // 50MB
	size := int64(len(testdata))
	reader := bytes.NewReader(testdata)

	server, client := testGenerateConnPair(t)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		n, err := io.CopyBuffer(server, reader, make([]byte, 128*1024))
		require.NoError(t, err)
		require.Equal(t, size, n)
	}()

	buffer := new(bytes.Buffer)
	n, err := io.CopyN(buffer, client, size)
	require.NoError(t, err)
	require.Equal(t, size, n)

	wg.Wait()

	require.True(t, bytes.Equal(testdata, buffer.Bytes()))

	err = server.Close()
	require.NoError(t, err)
	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, server)
	testsuite.IsDestroyed(t, client)
}

func TestConn_ReadZeroData(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, client := testGenerateConnPair(t)

	n, err := client.Read(nil)
	require.NoError(t, err)
	require.Equal(t, 0, n)

	err = server.Close()
	require.NoError(t, err)
	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, server)
	testsuite.IsDestroyed(t, client)
}

func TestConn_WriteZeroData(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, client := testGenerateConnPair(t)

	n, err := client.Write(nil)
	require.NoError(t, err)
	require.Equal(t, 0, n)

	err = server.Close()
	require.NoError(t, err)
	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, server)
	testsuite.IsDestroyed(t, client)
}

type fakeSender struct{}

func (fs fakeSender) Send(context.Context, []byte) error {
	return errors.New("error")
}

func TestConn_FailedToSend(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	cGUID := new(guid.GUID)
	cPort := uint32(1)
	sGUID := new(guid.GUID)
	sPort := uint32(2)
	err := cGUID.Write(bytes.Repeat([]byte{1}, guid.Size))
	require.NoError(t, err)
	err = sGUID.Write(bytes.Repeat([]byte{2}, guid.Size))
	require.NoError(t, err)
	client := NewConn(fakeSender{}, nil, cGUID, cPort, sGUID, sPort)

	n, err := client.Write(make([]byte, 128*1024))
	require.EqualError(t, err, "error")
	require.Equal(t, 0, n)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}
