package light

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestListenAndDial(t *testing.T) {
	gm := testsuite.MarkGoRoutines(t)
	defer gm.Compare()

	if testsuite.IPv4Enabled {
		testListenAndDial(t, "tcp4")
	}
	if testsuite.IPv6Enabled {
		testListenAndDial(t, "tcp6")
	}
}

func testListenAndDial(t *testing.T, network string) {
	listener, err := Listen(network, "localhost:0", 0)
	require.NoError(t, err)
	address := listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		return Dial(network, address, 0, nil)
	}, true)
}

func TestListenAndDialContext(t *testing.T) {
	gm := testsuite.MarkGoRoutines(t)
	defer gm.Compare()

	if testsuite.IPv4Enabled {
		testListenAndDialContext(t, "tcp4")
	}
	if testsuite.IPv6Enabled {
		testListenAndDialContext(t, "tcp6")
	}
}

func testListenAndDialContext(t *testing.T, network string) {
	listener, err := Listen(network, "localhost:0", 0)
	require.NoError(t, err)
	address := listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		return DialContext(ctx, network, address, 0, nil)
	}, true)
}

func TestDialContext_Timeout(t *testing.T) {
	gm := testsuite.MarkGoRoutines(t)
	defer gm.Compare()

	const network = "tcp"

	// failed to dialContext
	address := "0.0.0.1:0"
	_, err := Dial(network, address, time.Second, nil)
	require.Error(t, err)

	// handshake timeout
	listener, err := Listen(network, "localhost:0", 0)
	require.NoError(t, err)
	address = listener.Addr().String()
	_, err = Dial(network, address, time.Second, nil)
	require.Error(t, err)
}

func TestDialContext_Cancel(t *testing.T) {
	gm := testsuite.MarkGoRoutines(t)
	defer gm.Compare()

	const network = "tcp"

	listener, err := Listen(network, "localhost:0", 0)
	require.NoError(t, err)
	address := listener.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(time.Second)
		cancel()
	}()
	_, err = DialContext(ctx, network, address, 0, nil)
	require.Error(t, err)

	wg.Wait()
}

func TestFailedToListen(t *testing.T) {
	_, err := Listen("tcp", "foo address", 0)
	require.Error(t, err)
}
