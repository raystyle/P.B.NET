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
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
)

const mockListenerAcceptTimes = 10

// errors and panics about mock.
var (
	errMockConnRead               = errors.New("error in mockConn.Read()")
	mockConnReadPanic             = "panic in mockConn.Read()"
	errMockConnWrite              = errors.New("error in mockConn.Write()")
	mockConnWritePanic            = "panic in mockConn.Write()"
	errMockConnClose              = errors.New("error in mockConn.Close()")
	mockConnSetDeadlinePanic      = "panic in mockConn.SetDeadline()"
	mockConnSetReadDeadlinePanic  = "panic in mockConn.SetReadDeadline()"
	mockConnSetWriteDeadlinePanic = "panic in mockConn.SetWriteDeadline()"

	errMockListenerAccept      = &mockNetError{temporary: true}
	errMockListenerAcceptFatal = errors.New("mockListener.Accept() fatal")
	mockListenerAcceptPanic    = "panic in mockListener.Accept()"
	errMockListenerClose       = errors.New("error in mockListener.Close()")
	errMockListenerClosed      = errors.New("mock listener closed")

	errMockContext = errors.New("error in mockContext.Err()")

	errMockReadCloser = errors.New("error in mockReadCloser")
)

// mockNetError implement net.Error.
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

type mockConnLocalAddr struct{}

func (mockConnLocalAddr) Network() string {
	return "mock Conn local network"
}

func (mockConnLocalAddr) String() string {
	return "mock Conn local address"
}

type mockConnRemoteAddr struct{}

func (mockConnRemoteAddr) Network() string {
	return "mock Conn remote network"
}

func (mockConnRemoteAddr) String() string {
	return "mock Conn remote address"
}

type mockConn struct {
	local  mockConnLocalAddr
	remote mockConnRemoteAddr

	readError          bool // Read() error
	readPanic          bool // Read() panic
	writeError         bool // Write() error
	writePanic         bool // Write() panic
	closeError         bool // Close() error
	deadlinePanic      bool // SetDeadline() panic
	readDeadlinePanic  bool // SetReadDeadline() panic
	writeDeadlinePanic bool // SetWriteDeadline() panic

	ctx    context.Context
	cancel context.CancelFunc
}

func (c *mockConn) Read([]byte) (int, error) {
	if c.readError {
		return 0, errMockConnRead
	}
	if c.readPanic {
		panic(mockConnReadPanic)
	}
	if c.closeError {
		<-c.ctx.Done()
		return 0, errMockConnClose
	}
	return 0, nil
}

func (c *mockConn) Write([]byte) (int, error) {
	if c.writeError {
		return 0, errMockConnWrite
	}
	if c.writePanic {
		panic(mockConnWritePanic)
	}
	return 0, nil
}

func (c *mockConn) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	if c.closeError {
		return errMockConnClose
	}
	return nil
}

func (c *mockConn) LocalAddr() net.Addr {
	return c.local
}

func (c *mockConn) RemoteAddr() net.Addr {
	return c.remote
}

func (c *mockConn) SetDeadline(time.Time) error {
	if c.deadlinePanic {
		panic(mockConnSetDeadlinePanic)
	}
	return nil
}

func (c *mockConn) SetReadDeadline(time.Time) error {
	if c.readDeadlinePanic {
		panic(mockConnSetReadDeadlinePanic)
	}
	return nil
}

func (c *mockConn) SetWriteDeadline(time.Time) error {
	if c.writeDeadlinePanic {
		panic(mockConnSetWriteDeadlinePanic)
	}
	return nil
}

// NewMockConnWithReadError is used to create a mock conn that
// will return a errMockConnRead when call Read().
func NewMockConnWithReadError() net.Conn {
	return &mockConn{readError: true}
}

// IsMockConnReadError is used to check err is errMockConnRead.
func IsMockConnReadError(t testing.TB, err error) {
	require.Equal(t, errMockConnRead, err)
}

// NewMockConnWithReadPanic is used to create a mock conn that
// will return a mockConnReadPanic when call Read().
func NewMockConnWithReadPanic() net.Conn {
	return &mockConn{readPanic: true}
}

// IsMockConnReadPanic is used to check err.Error() is mockConnReadPanic.
func IsMockConnReadPanic(t testing.TB, err error) {
	require.Contains(t, err.Error(), mockConnReadPanic)
}

// NewMockConnWithWriteError is used to create a mock conn that
// will return a errMockConnWrite when call Write().
func NewMockConnWithWriteError() net.Conn {
	return &mockConn{writeError: true}
}

// IsMockConnWriteError is used to check err is errMockConnWrite.
func IsMockConnWriteError(t testing.TB, err error) {
	require.Equal(t, errMockConnWrite, err)
}

// NewMockConnWithWritePanic is used to create a mock conn that
// will return a mockConnWritePanic when call Write().
func NewMockConnWithWritePanic() net.Conn {
	return &mockConn{writePanic: true}
}

// IsMockConnWritePanic is used to check err.Error() is mockConnWritePanic.
func IsMockConnWritePanic(t testing.TB, err error) {
	require.Contains(t, err.Error(), mockConnWritePanic)
}

// NewMockConnWithCloseError is used to create a mock conn
// that will return a errMockConnClose when call Close().
func NewMockConnWithCloseError() net.Conn {
	conn := &mockConn{closeError: true}
	conn.ctx, conn.cancel = context.WithCancel(context.Background())
	return conn
}

// IsMockConnCloseError is used to check err is errMockConnClose.
func IsMockConnCloseError(t testing.TB, err error) {
	require.Equal(t, errMockConnClose, err)
}

// NewMockConnWithSetDeadlinePanic is used to create a mock conn
// that will panic when call SetDeadline().
func NewMockConnWithSetDeadlinePanic() net.Conn {
	return &mockConn{deadlinePanic: true}
}

// IsMockConnSetDeadlinePanic is used to check err.Error() is mockConnSetDeadlinePanic.
func IsMockConnSetDeadlinePanic(t testing.TB, err error) {
	require.Contains(t, err.Error(), mockConnSetDeadlinePanic)
}

// NewMockConnWithSetReadDeadlinePanic is used to create a mock conn
// that will panic when call SetReadDeadline().
func NewMockConnWithSetReadDeadlinePanic() net.Conn {
	return &mockConn{readDeadlinePanic: true}
}

// IsMockConnSetReadDeadlinePanic is used to check err.Error() is mockConnSetReadDeadlinePanic.
func IsMockConnSetReadDeadlinePanic(t testing.TB, err error) {
	require.Contains(t, err.Error(), mockConnSetReadDeadlinePanic)
}

// NewMockConnWithSetWriteDeadlinePanic is used to create a mock conn
// that will panic when call SetWriteDeadline().
func NewMockConnWithSetWriteDeadlinePanic() net.Conn {
	return &mockConn{writeDeadlinePanic: true}
}

// IsMockConnSetWriteDeadlinePanic is used to check err.Error() is mockConnSetWriteDeadlinePanic.
func IsMockConnSetWriteDeadlinePanic(t testing.TB, err error) {
	require.Contains(t, err.Error(), mockConnSetWriteDeadlinePanic)
}

type mockListenerAddr struct{}

func (mockListenerAddr) Network() string {
	return "mock Listener network"
}

func (mockListenerAddr) String() string {
	return "mock Listener address"
}

type mockListener struct {
	addr mockListenerAddr

	error bool // Accept() error
	panic bool // Accept() panic
	n     int  // Accept() count
	close bool // Close() error

	ctx    context.Context
	cancel context.CancelFunc
}

func (l *mockListener) Accept() (net.Conn, error) {
	if l.n > mockListenerAcceptTimes {
		return nil, errMockListenerAcceptFatal
	}
	l.n++
	if l.error {
		return nil, errMockListenerAccept
	}
	if l.panic {
		panic(mockListenerAcceptPanic)
	}
	if l.close {
		<-l.ctx.Done()
		return nil, errMockListenerClosed
	}
	return nil, nil
}

func (l *mockListener) Close() error {
	if l.cancel != nil {
		l.cancel()
	}
	if l.close {
		return errMockListenerClose
	}
	return nil
}

func (l *mockListener) Addr() net.Addr {
	return l.addr
}

// NewMockListenerWithAcceptError is used to create a mock listener
// that return a temporary error call Accept().
func NewMockListenerWithAcceptError() net.Listener {
	l := &mockListener{error: true}
	return l
}

// IsMockListenerAcceptFatal is used to check err is errMockListenerAcceptFatal.
func IsMockListenerAcceptFatal(t testing.TB, err error) {
	require.Equal(t, errMockListenerAcceptFatal, err)
}

// NewMockListenerWithAcceptPanic is used to create a mock listener
// that panic when call Accept().
func NewMockListenerWithAcceptPanic() net.Listener {
	l := &mockListener{panic: true}
	return l
}

// IsMockListenerAcceptPanic is used to check err.Error() is mockListenerAcceptPanic.
func IsMockListenerAcceptPanic(t testing.TB, err error) {
	require.Contains(t, err.Error(), mockListenerAcceptPanic)
}

// NewMockListenerWithCloseError is used to create a mock listener
// that will return a errMockListenerClose when call Close().
func NewMockListenerWithCloseError() net.Listener {
	l := &mockListener{close: true}
	l.ctx, l.cancel = context.WithCancel(context.Background())
	return l
}

// IsMockListenerCloseError is used to check err is errMockListenerClose.
func IsMockListenerCloseError(t testing.TB, err error) {
	require.Equal(t, errMockListenerClose, err)
}

// IsMockListenerClosedError is used to check err is errMockListenerClosed.
func IsMockListenerClosedError(t testing.TB, err error) {
	require.Equal(t, errMockListenerClosed, err)
}

type mockContext struct {
	done  chan struct{}
	error bool
}

func (*mockContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (ctx *mockContext) Done() <-chan struct{} {
	return ctx.done
}

func (ctx *mockContext) Err() error {
	if ctx.error {
		return errMockContext
	}
	return nil
}

func (*mockContext) Value(interface{}) interface{} {
	return nil
}

// NewMockContextWithError is used to create a context with error.
func NewMockContextWithError() (context.Context, context.CancelFunc) {
	done := make(chan struct{})
	once := sync.Once{}
	ctx := mockContext{
		done:  done,
		error: true,
	}
	cancel := func() {
		once.Do(func() {
			close(done)
		})
	}
	return &ctx, cancel
}

// IsMockContextError is used to check err is errMockContext.
func IsMockContextError(t testing.TB, err error) {
	require.Equal(t, errMockContext, err)
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

// TODO renamed
type mockConnReadPanic2 struct {
	net.Conn
	server net.Conn
}

func (c *mockConnReadPanic2) Read([]byte) (int, error) {
	panic("mock panic in Read()")
}

func (c *mockConnReadPanic2) Close() error {
	_ = c.Conn.Close()
	_ = c.server.Close()
	return nil
}

// DialMockConnWithReadPanic is used to create a mock connection
// and when call Read() it will panic.
func DialMockConnWithReadPanic(_ context.Context, _, _ string) (net.Conn, error) {
	server, client := net.Pipe()
	go func() { _, _ = io.Copy(ioutil.Discard, server) }()
	return &mockConnReadPanic2{
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
