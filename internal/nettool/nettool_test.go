package nettool

import (
	"bytes"
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
)

func TestCheckPort(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		err := CheckPort(123)
		require.NoError(t, err)
	})

	t.Run("invalid port", func(t *testing.T) {
		err := CheckPort(-1)
		require.EqualError(t, err, "invalid port: -1")
		err = CheckPort(65536)
		require.EqualError(t, err, "invalid port: 65536")
	})
}

func TestCheckPortString(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		err := CheckPortString("1234")
		require.NoError(t, err)
	})

	t.Run("empty port", func(t *testing.T) {
		err := CheckPortString("")
		require.Equal(t, ErrEmptyPort, err)
	})

	t.Run("NaN", func(t *testing.T) {
		err := CheckPortString("s")
		require.Error(t, err)
	})

	t.Run("invalid port", func(t *testing.T) {
		err := CheckPortString("-1")
		require.Error(t, err)
		err = CheckPortString("65536")
		require.Error(t, err)
	})
}

func TestJoinHostPort(t *testing.T) {
	t.Run("IPv4", func(t *testing.T) {
		address := JoinHostPort("1.1.1.1", 123)
		require.Equal(t, "1.1.1.1:123", address)
	})

	t.Run("IPv6", func(t *testing.T) {
		address := JoinHostPort("::1", 123)
		require.Equal(t, "[::1]:123", address)
	})
}

func TestSplitHostPort(t *testing.T) {
	t.Run("host and port", func(t *testing.T) {
		host, port, err := SplitHostPort("host:123")
		require.NoError(t, err)
		require.Equal(t, "host", host)
		require.Equal(t, uint16(123), port)
	})

	t.Run("miss port", func(t *testing.T) {
		_, _, err := SplitHostPort("host")
		require.Error(t, err)
	})

	t.Run("port is NaN", func(t *testing.T) {
		_, _, err := SplitHostPort("host:NaN")
		require.Error(t, err)
	})

	t.Run("invalid port", func(t *testing.T) {
		_, _, err := SplitHostPort("host:99999")
		require.Error(t, err)
	})
}

func TestIPToHost(t *testing.T) {
	t.Run("IPv4", func(t *testing.T) {
		host := IPToHost("127.0.0.1")
		require.Equal(t, "127.0.0.1", host)
	})

	t.Run("IPv6", func(t *testing.T) {
		host := IPToHost("::1")
		require.Equal(t, "[::1]", host)
	})
}

func TestIsTCPNetwork(t *testing.T) {
	t.Run("is", func(t *testing.T) {
		err := IsTCPNetwork("tcp")
		require.NoError(t, err)
		err = IsTCPNetwork("tcp4")
		require.NoError(t, err)
		err = IsTCPNetwork("tcp6")
		require.NoError(t, err)
	})

	t.Run("not", func(t *testing.T) {
		err := IsTCPNetwork("foo")
		require.EqualError(t, err, "invalid tcp network: foo")
	})
}

func TestIsUDPNetwork(t *testing.T) {
	t.Run("is", func(t *testing.T) {
		err := IsUDPNetwork("udp")
		require.NoError(t, err)
		err = IsUDPNetwork("udp4")
		require.NoError(t, err)
		err = IsUDPNetwork("udp6")
		require.NoError(t, err)
	})

	t.Run("not", func(t *testing.T) {
		err := IsUDPNetwork("foo")
		require.EqualError(t, err, "invalid udp network: foo")
	})
}

func TestIsNetClosingError(t *testing.T) {
	t.Run("is", func(t *testing.T) {
		err := errors.New("test error: use of closed network connection")
		r := IsNetClosingError(err)
		require.True(t, r)
	})

	t.Run("nil error", func(t *testing.T) {
		r := IsNetClosingError(nil)
		require.False(t, r)
	})

	t.Run("not", func(t *testing.T) {
		err := errors.New("test error")
		r := IsNetClosingError(err)
		require.False(t, r)
	})
}

func TestEncodeExternalAddress(t *testing.T) {
	t.Run("IPv4", func(t *testing.T) {
		ip := EncodeExternalAddress("1.2.3.4:80")
		require.Equal(t, net.IPv4(1, 2, 3, 4), net.IP(ip))
	})

	t.Run("IPv6", func(t *testing.T) {
		ip := EncodeExternalAddress("[::]:80")
		require.Equal(t, []byte(net.IPv6zero), ip)
	})

	t.Run("domain", func(t *testing.T) {
		host := EncodeExternalAddress("domain:80")
		require.Equal(t, []byte("domain"), host)
	})

	t.Run("other", func(t *testing.T) {
		host := EncodeExternalAddress("other")
		require.Equal(t, []byte("other"), host)
	})
}

func TestDecodeExternalAddress(t *testing.T) {
	t.Run("IPv4", func(t *testing.T) {
		ip := DecodeExternalAddress([]byte{1, 2, 3, 4})
		require.Equal(t, "1.2.3.4", ip)
	})

	t.Run("IPv6", func(t *testing.T) {
		ip := DecodeExternalAddress(net.IPv6zero)
		require.Equal(t, "::", ip)
	})

	t.Run("domain", func(t *testing.T) {
		host := DecodeExternalAddress([]byte("domain"))
		require.Equal(t, "domain", host)
	})
}

func TestIPEnabled(t *testing.T) {
	t.Run("current", func(t *testing.T) {
		ipv4, ipv6 := IPEnabled()
		t.Log("current:", ipv4, ipv6)
	})

	t.Run("failed to get interfaces", func(t *testing.T) {
		patch := func() ([]net.Interface, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(net.Interfaces, patch)
		defer pg.Unpatch()

		ipv4, ipv6 := IPEnabled()
		require.False(t, ipv4)
		require.False(t, ipv6)
	})

	t.Run("fake IPv4 Only", func(t *testing.T) {
		patch := func(string) net.IP {
			return bytes.Repeat([]byte{1}, net.IPv4len)
		}
		pg := monkey.Patch(net.ParseIP, patch)
		defer pg.Unpatch()

		ipv4, ipv6 := IPEnabled()
		require.True(t, ipv4)
		require.False(t, ipv6)
	})

	t.Run("fake IPv6 Only", func(t *testing.T) {
		patch := func(string) net.IP {
			return bytes.Repeat([]byte{1}, net.IPv6len)
		}
		pg := monkey.Patch(net.ParseIP, patch)
		defer pg.Unpatch()

		ipv4, ipv6 := IPEnabled()
		require.False(t, ipv4)
		require.True(t, ipv6)
	})

	t.Run("fake double stack", func(t *testing.T) {
		called := false
		patch := func(string) net.IP {
			if called {
				return bytes.Repeat([]byte{1}, net.IPv4len)
			}
			called = true
			return bytes.Repeat([]byte{1}, net.IPv6len)
		}
		pg := monkey.Patch(net.ParseIP, patch)
		defer pg.Unpatch()

		ipv4, ipv6 := IPEnabled()
		require.True(t, ipv4)
		require.True(t, ipv6)
	})
}

func TestDeadlineConn(t *testing.T) {
	server, client := net.Pipe()
	client = DeadlineConn(client, 100*time.Millisecond)
	server = DeadlineConn(server, 100*time.Millisecond)

	// deadline
	buf := make([]byte, 1024)
	_, err := client.Read(buf)
	require.Error(t, err)
	_, err = client.Write(buf)
	require.Error(t, err)
	err = client.Close()
	require.NoError(t, err)
	err = server.Close()
	require.NoError(t, err)

	// default deadline
	server, client = net.Pipe()
	client = DeadlineConn(client, 0)
	server = DeadlineConn(server, 0)
	err = client.Close()
	require.NoError(t, err)
	err = server.Close()
	require.NoError(t, err)
}

type mockServer struct {
	addresses    []net.Addr
	addressesRWM sync.RWMutex
}

func (srv *mockServer) Serve() {
	srv.addressesRWM.Lock()
	defer srv.addressesRWM.Unlock()
	addr := net.TCPAddr{
		IP:   net.IPv4zero,
		Port: 1234,
	}
	srv.addresses = append(srv.addresses, &addr)
}

func (srv *mockServer) Addresses() []net.Addr {
	srv.addressesRWM.RLock()
	defer srv.addressesRWM.RUnlock()
	return srv.addresses
}

func TestWaitServerServe(t *testing.T) {
	t.Run("common", func(t *testing.T) {
		server := new(mockServer)

		errCh := make(chan error, 1)
		go func() { // mock
			server.Serve()
			errCh <- nil
		}()
		addrs, err := WaitServerServe(context.Background(), errCh, server, 1)
		require.NoError(t, err)
		require.Len(t, addrs, 1)
	})

	t.Run("error", func(t *testing.T) {
		server := new(mockServer)

		errCh := make(chan error, 1)
		go func() {
			errCh <- errors.New("test")
		}()
		addrs, err := WaitServerServe(context.Background(), errCh, server, 1)
		require.EqualError(t, err, "test")
		require.Nil(t, addrs)
	})

	t.Run("timeout", func(t *testing.T) {
		server := new(mockServer)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		addrs, err := WaitServerServe(ctx, nil, server, 1)
		require.Equal(t, context.DeadlineExceeded, err)
		require.Nil(t, addrs)
	})

	t.Run("canceled", func(t *testing.T) {
		server := new(mockServer)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		cancel()
		addrs, err := WaitServerServe(ctx, nil, server, 1)
		require.Equal(t, context.Canceled, err)
		require.Nil(t, addrs)
	})

	t.Run("invalid n", func(t *testing.T) {
		server := new(mockServer)

		defer func() {
			require.NotNil(t, recover())
		}()
		_, _ = WaitServerServe(context.Background(), nil, server, 0)
	})
}
