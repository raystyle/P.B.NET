package testsuite

import (
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ListenerAndDial is used to test net.Listener and Dial
func ListenerAndDial(t testing.TB, l net.Listener, d func() (net.Conn, error), close bool) {
	wg := sync.WaitGroup{}
	for i := 0; i < 3; i++ {
		var server net.Conn
		wg.Add(1)
		go func() {
			defer wg.Done()
			var err error
			server, err = l.Accept()
			require.NoError(t, err)
		}()
		client, err := d()
		require.NoError(t, err)
		wg.Wait()
		Conn(t, server, client, close)
		t.Log("") // new line for Conn
	}
	require.NoError(t, l.Close())
	IsDestroyed(t, l)
}

// Conn is used to test client & server Conn
//
// if close == true, IsDestroyed will be run after Conn.Close()
// if Conn about TLS and use net.Pipe(), set close = false
// server, client := net.Pipe()
// tlsServer = tls.Server(server, tlsConfig)
// tlsClient = tls.Client(client, tlsConfig)
// Conn(t, tlsServer, tlsClient, false) must set false
func Conn(t testing.TB, server, client net.Conn, close bool) {
	t.Log("server remote:", server.RemoteAddr().Network(), server.RemoteAddr())
	t.Log("client local:", client.LocalAddr().Network(), client.LocalAddr())
	t.Log("server local:", server.LocalAddr().Network(), server.LocalAddr())
	t.Log("client remote:", client.RemoteAddr().Network(), client.RemoteAddr())

	// skip udp, because client.LocalAddr() always net.IPv4zero or net.IPv6zero
	if !strings.Contains(server.RemoteAddr().Network(), "udp") {
		require.Equal(t, server.RemoteAddr().Network(), client.LocalAddr().Network())
		require.Equal(t, server.RemoteAddr().String(), client.LocalAddr().String())
	}
	require.Equal(t, server.LocalAddr().Network(), client.RemoteAddr().Network())
	require.Equal(t, server.LocalAddr().String(), client.RemoteAddr().String())

	// Read() and Write()
	write := func(conn net.Conn) {
		data := Bytes()
		_, err := conn.Write(data)
		require.NoError(t, err)
		require.Equal(t, Bytes(), data)
	}
	read := func(conn net.Conn) {
		data := make([]byte, 256)
		_, err := io.ReadFull(conn, data)
		require.NoError(t, err)
		require.Equal(t, Bytes(), data)
	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	// server
	go func() {
		defer wg.Done()
		read(server)
		write(server)
		write(server)
		read(server)
	}()
	// client
	write(client)
	read(client)
	read(client)
	write(client)
	wg.Wait()

	// about Deadline()
	require.NoError(t, server.SetDeadline(time.Now().Add(10*time.Millisecond)))
	require.NoError(t, client.SetDeadline(time.Now().Add(10*time.Millisecond)))
	time.Sleep(20 * time.Millisecond)
	buf := []byte{0, 0, 0, 0}
	_, err := client.Write(buf)
	require.Error(t, err)
	_, err = server.Read(buf)
	require.Error(t, err)

	require.NoError(t, server.SetReadDeadline(time.Now().Add(10*time.Millisecond)))
	require.NoError(t, client.SetWriteDeadline(time.Now().Add(10*time.Millisecond)))
	time.Sleep(20 * time.Millisecond)
	_, err = client.Write(buf)
	require.Error(t, err)
	_, err = server.Read(buf)
	require.Error(t, err)

	// recovery deadline
	require.NoError(t, server.SetDeadline(time.Time{}))
	require.NoError(t, client.SetDeadline(time.Time{}))

	// Close()
	if close {
		require.NoError(t, server.Close())
		require.NoError(t, client.Close())

		IsDestroyed(t, server)
		IsDestroyed(t, client)
	}
}
