package nettool

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCheckPort(t *testing.T) {
	err := CheckPort(123)
	require.NoError(t, err)

	// invalid port
	err = CheckPort(-1)
	require.EqualError(t, err, "invalid port: -1")
	err = CheckPort(65536)
	require.EqualError(t, err, "invalid port: 65536")
}

func TestCheckPortString(t *testing.T) {
	err := CheckPortString("1234")
	require.NoError(t, err)

	err = CheckPortString("")
	require.Equal(t, ErrEmptyPort, err)

	// NaN
	err = CheckPortString("s")
	require.Error(t, err)

	// invalid port
	err = CheckPortString("-1")
	require.Error(t, err)
	err = CheckPortString("65536")
	require.Error(t, err)
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
	t.Log(IPEnabled())
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
