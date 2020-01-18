package testsuite

import (
	"bufio"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// about mock listener Accept()
var (
	ErrMockListener   = errors.New("mock listener accept error")
	MockListenerPanic = "mock listener accept panic"
)

type mockListenerAddr struct{}

func (mockListenerAddr) Network() string {
	return "mockListenerAddr-network"
}

func (mockListenerAddr) String() string {
	return "mockListenerAddr-String"
}

type mockListener struct {
	error bool
	panic bool
}

func (l mockListener) Accept() (net.Conn, error) {
	if l.error {
		return nil, ErrMockListener
	}
	if l.panic {
		panic(MockListenerPanic)
	}
	return nil, nil
}

func (l mockListener) Close() error {
	return nil
}

func (l mockListener) Addr() net.Addr {
	return new(mockListenerAddr)
}

// NewMockListenerWithError is used to create a mock listener
// that return a custom error call Accept()
func NewMockListenerWithError() net.Listener {
	return &mockListener{error: true}
}

// IsMockListenerError is used to confirm err is ErrMockListener
func IsMockListenerError(t testing.TB, err error) {
	require.Equal(t, ErrMockListener, err)
}

// NewMockListenerWithPanic is used to create a mock listener
// that panic when call Accept()
func NewMockListenerWithPanic() net.Listener {
	return &mockListener{panic: true}
}

// IsMockListenerPanic is used to confirm err.Error() is MockListenerPanic
func IsMockListenerPanic(t testing.TB, err error) {
	require.Contains(t, err.Error(), MockListenerPanic)
}

type mockResponseWriter struct {
	hijack bool
	conn   net.Conn
}

func (mockResponseWriter) Header() http.Header {
	return nil
}

func (mockResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (mockResponseWriter) WriteHeader(int) {}

func (rw mockResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if rw.hijack {
		return nil, nil, errors.New("failed to hijack")
	}
	return rw.conn, nil, nil
}

type mockConn struct {
	net.Conn
	server net.Conn
}

func (c mockConn) Read([]byte) (int, error) {
	defer func() { panic("Read() panic") }()
	return 0, nil
}

func (c mockConn) Close() error {
	defer func() { panic("Close() panic") }()
	_ = c.Conn.Close()
	_ = c.server.Close()
	return nil
}

// NewMockResponseWriterWithFailedToHijack is used to create a mock
// http.ResponseWriter that implemented http.Hijacker, if call Hijack()
// it will return an error
func NewMockResponseWriterWithFailedToHijack() http.ResponseWriter {
	return &mockResponseWriter{hijack: true}
}

// NewMockResponseWriterWithFailedToWrite is used to create a mock
// http.ResponseWriter that implemented http.Hijacker, if use hijacked
// connection, it will return an error
func NewMockResponseWriterWithFailedToWrite() http.ResponseWriter {
	server, client := net.Pipe()
	_ = client.Close()
	_ = server.Close()
	return &mockResponseWriter{conn: client}
}

// NewMockResponseWriterWithMockConn is used to create a mock
// http.ResponseWriter that implemented http.Hijacker, if use hijacked
// connection, call Read() or Close(), it will panic
func NewMockResponseWriterWithMockConn() http.ResponseWriter {
	server, client := net.Pipe()
	go func() { _, _ = io.Copy(ioutil.Discard, server) }()
	mc := mockConn{
		Conn:   client,
		server: server,
	}
	return &mockResponseWriter{conn: &mc}
}
