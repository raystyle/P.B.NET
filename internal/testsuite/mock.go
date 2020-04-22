package testsuite

import (
	"bufio"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
)

// about mock listener Accept() and other
var (
	errMockListenerAccept = &mockNetError{temporary: true}
	errMockListener       = errors.New("accept more than 10 times")
	mockListenerPanic     = "mock panic in listener Accept()"
	errMockReadCloser     = errors.New("mock error in io.ReadCloser")
)

// mockNetError implement net.Error
type mockNetError struct {
	timeout   bool
	temporary bool
}

func (*mockNetError) Error() string {
	return "mock net error"
}

func (e *mockNetError) Timeout() bool {
	return e.timeout
}

func (e *mockNetError) Temporary() bool {
	return e.temporary
}

type mockListenerAddr struct{}

func (mockListenerAddr) Network() string {
	return "mock Listener network"
}

func (mockListenerAddr) String() string {
	return "mock Listener address"
}

type mockListener struct {
	error bool
	panic bool
	n     int
}

func (l *mockListener) Accept() (net.Conn, error) {
	if l.n > 10 {
		return nil, errMockListener
	}
	l.n++
	if l.error {
		return nil, errMockListenerAccept
	}
	if l.panic {
		panic(mockListenerPanic)
	}
	return nil, nil
}

func (l *mockListener) Close() error {
	return nil
}

func (l *mockListener) Addr() net.Addr {
	return new(mockListenerAddr)
}

// NewMockListenerWithError is used to create a mock listener
// that return a custom error call Accept().
func NewMockListenerWithError() net.Listener {
	return &mockListener{error: true}
}

// NewMockListenerWithPanic is used to create a mock listener.
// that panic when call Accept()
func NewMockListenerWithPanic() net.Listener {
	return &mockListener{panic: true}
}

// IsMockListenerError is used to confirm err is errMockListenerAccept.
func IsMockListenerError(t testing.TB, err error) {
	require.Equal(t, errMockListener, err)
}

// IsMockListenerPanic is used to confirm err.Error() is mockListenerPanic.
func IsMockListenerPanic(t testing.TB, err error) {
	require.Contains(t, err.Error(), mockListenerPanic)
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

// NewMockResponseWriter is used to create simple mock response writer.
func NewMockResponseWriter() http.ResponseWriter {
	server, client := net.Pipe()
	go func() { _, _ = io.Copy(ioutil.Discard, server) }()
	return &mockResponseWriter{conn: client}
}

// NewMockResponseWriterWithFailedToHijack is used to create a mock
// http.ResponseWriter that implemented http.Hijacker, if call Hijack()
// it will return an error.
func NewMockResponseWriterWithFailedToHijack() http.ResponseWriter {
	return &mockResponseWriter{hijack: true}
}

// NewMockResponseWriterWithFailedToWrite is used to create a mock
// http.ResponseWriter that implemented http.Hijacker, if use hijacked
// connection, it will return an error.
func NewMockResponseWriterWithFailedToWrite() http.ResponseWriter {
	server, client := net.Pipe()
	_ = client.Close()
	_ = server.Close()
	return &mockResponseWriter{conn: client}
}

type mockConnClosePanic struct {
	net.Conn
	server net.Conn
}

func (c *mockConnClosePanic) Close() error {
	defer func() { panic("mock panic in Close()") }()
	_ = c.Conn.Close()
	_ = c.server.Close()
	return nil
}

// NewMockResponseWriterWithClosePanic is used to create a mock
// http.ResponseWriter that implemented http.Hijacker, if use hijacked
// connection and when call Close() it will panic.
func NewMockResponseWriterWithClosePanic() http.ResponseWriter {
	server, client := net.Pipe()
	go func() { _, _ = io.Copy(ioutil.Discard, server) }()
	mc := mockConnClosePanic{
		Conn:   client,
		server: server,
	}
	return &mockResponseWriter{conn: &mc}
}

type mockConnReadPanic struct {
	net.Conn
	server net.Conn
}

func (c *mockConnReadPanic) Read([]byte) (int, error) {
	panic("mock panic in Read()")
}

func (c *mockConnReadPanic) Close() error {
	_ = c.Conn.Close()
	_ = c.server.Close()
	return nil
}

// DialMockConnWithReadPanic is used to create a mock connection
// and when call Read() it will panic.
func DialMockConnWithReadPanic(_ context.Context, _, _ string) (net.Conn, error) {
	server, client := net.Pipe()
	go func() { _, _ = io.Copy(ioutil.Discard, server) }()
	return &mockConnReadPanic{
		Conn:   client,
		server: server,
	}, nil
}

type mockConnWriteError struct {
	net.Conn
	server net.Conn
}

func (c *mockConnWriteError) Read(b []byte) (int, error) {
	b[0] = 1
	return 1, nil
}

func (c *mockConnWriteError) Write([]byte) (int, error) {
	return 0, monkey.Error
}

func (c *mockConnWriteError) Close() error {
	_ = c.Conn.Close()
	_ = c.server.Close()
	return nil
}

// DialMockConnWithWriteError is used to create a mock connection
// and when call Write() it will return a monkey error.
func DialMockConnWithWriteError(_ context.Context, _, _ string) (net.Conn, error) {
	server, client := net.Pipe()
	go func() { _, _ = io.Copy(ioutil.Discard, server) }()
	return &mockConnWriteError{
		Conn:   client,
		server: server,
	}, nil
}

type mockReadCloser struct {
	panic bool
	rwm   sync.RWMutex
}

func (rc *mockReadCloser) Read([]byte) (int, error) {
	rc.rwm.RLock()
	defer rc.rwm.RUnlock()
	if rc.panic {
		panic("mock panic in Read()")
	}
	return 0, errMockReadCloser
}

func (rc *mockReadCloser) Close() error {
	rc.rwm.Lock()
	defer rc.rwm.Unlock()
	rc.panic = false
	return nil
}

// NewMockReadCloserWithReadError is used to return a ReadCloser that
// return a errMockReadCloser when call Read().
func NewMockReadCloserWithReadError() io.ReadCloser {
	return new(mockReadCloser)
}

// NewMockReadCloserWithReadPanic is used to return a ReadCloser that
// panic when call Read().
func NewMockReadCloserWithReadPanic() io.ReadCloser {
	return &mockReadCloser{panic: true}
}

// IsMockReadCloserError is used to confirm err is errMockReadCloser.
func IsMockReadCloserError(t testing.TB, err error) {
	require.Equal(t, errMockReadCloser, err)
}
