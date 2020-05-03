package light

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestConnWithBackground(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	testConnWithBackground(t, testsuite.ConnSC)
	testConnWithBackground(t, testsuite.ConnCS)
}

func testConnWithBackground(t *testing.T, f func(*testing.T, net.Conn, net.Conn, bool)) {
	server, client := net.Pipe()
	server = Server(context.Background(), server, 0)
	client = Client(context.Background(), client, 0)
	f(t, server, client, true)
}

func TestConnWithCancel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	testConnWithCancel(t, testsuite.ConnSC)
	testConnWithCancel(t, testsuite.ConnCS)
}

func testConnWithCancel(t *testing.T, f func(*testing.T, net.Conn, net.Conn, bool)) {
	server, client := net.Pipe()
	sCtx, sCancel := context.WithCancel(context.Background())
	defer sCancel()
	server = Server(sCtx, server, 0)
	cCtx, cCancel := context.WithCancel(context.Background())
	defer cCancel()
	client = Client(cCtx, client, 0)
	f(t, server, client, true)
}

func TestConn_Handshake_Timeout(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, client := net.Pipe()
	sCtx, sCancel := context.WithCancel(context.Background())
	defer sCancel()
	server = Server(sCtx, server, time.Second)
	cCtx, cCancel := context.WithCancel(context.Background())
	defer cCancel()
	client = Client(cCtx, client, time.Second)

	_, err := client.Read(make([]byte, 1))
	require.Error(t, err)
	_, err = server.Write(make([]byte, 1))
	require.Error(t, err)

	require.NoError(t, client.Close())
	require.NoError(t, server.Close())
	testsuite.IsDestroyed(t, client)
	testsuite.IsDestroyed(t, server)
}

func TestConn_Handshake_Cancel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, client := net.Pipe()
	sCtx, sCancel := context.WithCancel(context.Background())
	defer sCancel()
	server = Server(sCtx, server, 0)
	cCtx, cCancel := context.WithCancel(context.Background())
	defer cCancel()
	client = Client(cCtx, client, 0)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(100 * time.Millisecond)
		sCancel()
		cCancel()
	}()

	_, err := client.Read(make([]byte, 1))
	require.Error(t, err)
	_, err = server.Write(make([]byte, 1))
	require.Error(t, err)

	wg.Wait()

	require.NoError(t, client.Close())
	require.NoError(t, server.Close())
	testsuite.IsDestroyed(t, client)
	testsuite.IsDestroyed(t, server)
}

func TestConn_Handshake_Panic(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, client := net.Pipe()
	sCtx, sCancel := context.WithCancel(context.Background())
	defer sCancel()
	server = Server(sCtx, server, 0)
	cCtx, cCancel := context.WithCancel(context.Background())
	defer cCancel()
	client = Client(cCtx, client, 0)

	patch := func(_ interface{}) {
		panic(monkey.Panic)
	}
	pg := monkey.PatchInstanceMethod(sCtx, "Done", patch)
	defer pg.Unpatch()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := server.(*Conn).Handshake()
		require.NoError(t, err)
	}()

	time.Sleep(time.Second)
	err := client.(*Conn).Handshake()
	require.NoError(t, err)

	wg.Wait()

	require.NoError(t, client.Close())
	require.NoError(t, server.Close())
	testsuite.IsDestroyed(t, client)
	testsuite.IsDestroyed(t, server)
}
