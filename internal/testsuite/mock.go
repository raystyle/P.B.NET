package testsuite

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const mockListenerAcceptTimes = 10

// errors and panics about mock.
var (
	errMockConnRead               = errors.New("error in mockConn.Read()")
	mockConnReadPanic             = "panic in mockConn.Read()"
	errMockConnWrite              = errors.New("error in mockConn.Write()")
	mockConnWritePanic            = "panic in mockConn.Write()"
	errMockConnClose              = errors.New("error in mockConn.Close()")
	mockConnClosePanic            = "panic in mockConn.Close()"
	mockConnSetDeadlinePanic      = "panic in mockConn.SetDeadline()"
	mockConnSetReadDeadlinePanic  = "panic in mockConn.SetReadDeadline()"
	mockConnSetWriteDeadlinePanic = "panic in mockConn.SetWriteDeadline()"
	errMockConnClosed             = errors.New("mock conn closed")

	errMockListenerAccept      = &mockNetError{temporary: true}
	errMockListenerAcceptFatal = errors.New("mockListener.Accept() fatal")
	mockListenerAcceptPanic    = "panic in mockListener.Accept()"
	errMockListenerClose       = errors.New("error in mockListener.Close()")
	errMockListenerClosed      = errors.New("mock listener closed")

	errMockContext = errors.New("error in mockContext.Err()")
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
	closePanic         bool // Close() panic
	deadlinePanic      bool // SetDeadline() panic
	readDeadlinePanic  bool // SetReadDeadline() panic
	writeDeadlinePanic bool // SetWriteDeadline() panic

	closed int32

	ctx    context.Context
	cancel context.CancelFunc
}

func (c *mockConn) isClosed() bool {
	return atomic.LoadInt32(&c.closed) != 0
}

func (c *mockConn) Read([]byte) (int, error) {
	if c.isClosed() {
		return 0, errMockConnClosed
	}
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
	// prevent use too much CPU
	time.Sleep(100 * time.Millisecond)
	return 0, nil
}

func (c *mockConn) Write(b []byte) (int, error) {
	if c.isClosed() {
		return 0, errMockConnClosed
	}
	if c.writeError {
		return 0, errMockConnWrite
	}
	if c.writePanic {
		panic(mockConnWritePanic)
	}
	return len(b), nil
}

func (c *mockConn) Close() error {
	atomic.StoreInt32(&c.closed, 1)
	if c.cancel != nil {
		c.cancel()
	}
	if c.closeError {
		return errMockConnClose
	}
	if c.closePanic {
		panic(mockConnClosePanic)
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
	if c.isClosed() {
		return errMockConnClosed
	}
	if c.deadlinePanic {
		panic(mockConnSetDeadlinePanic)
	}
	return nil
}

func (c *mockConn) SetReadDeadline(time.Time) error {
	if c.isClosed() {
		return errMockConnClosed
	}
	if c.readDeadlinePanic {
		panic(mockConnSetReadDeadlinePanic)
	}
	return nil
}

func (c *mockConn) SetWriteDeadline(time.Time) error {
	if c.isClosed() {
		return errMockConnClosed
	}
	if c.writeDeadlinePanic {
		panic(mockConnSetWriteDeadlinePanic)
	}
	return nil
}

// NewMockConn is used to create a mock connection.
func NewMockConn() net.Conn {
	return new(mockConn)
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

// NewMockConnWithClosePanic is used to create a mock conn
// that will panic when call Close().
func NewMockConnWithClosePanic() net.Conn {
	return &mockConn{closePanic: true}
}

// IsMockConnClosePanic is used to check err.Error() is mockConnClosePanic.
func IsMockConnClosePanic(t testing.TB, err error) {
	require.Contains(t, err.Error(), mockConnClosePanic)
}

// NewMockConnWithSetDeadlinePanic is used to create a mock conn
// that will panic when call SetDeadline().
func NewMockConnWithSetDeadlinePanic() net.Conn {
	return &mockConn{deadlinePanic: true}
}

// IsMockConnSetDeadlinePanic is used to check err.Error()
// is mockConnSetDeadlinePanic.
func IsMockConnSetDeadlinePanic(t testing.TB, err error) {
	require.Contains(t, err.Error(), mockConnSetDeadlinePanic)
}

// NewMockConnWithSetReadDeadlinePanic is used to create a mock conn
// that will panic when call SetReadDeadline().
func NewMockConnWithSetReadDeadlinePanic() net.Conn {
	return &mockConn{readDeadlinePanic: true}
}

// IsMockConnSetReadDeadlinePanic is used to check err.Error()
// is mockConnSetReadDeadlinePanic.
func IsMockConnSetReadDeadlinePanic(t testing.TB, err error) {
	require.Contains(t, err.Error(), mockConnSetReadDeadlinePanic)
}

// NewMockConnWithSetWriteDeadlinePanic is used to create a mock conn
// that will panic when call SetWriteDeadline().
func NewMockConnWithSetWriteDeadlinePanic() net.Conn {
	return &mockConn{writeDeadlinePanic: true}
}

// IsMockConnSetWriteDeadlinePanic is used to check err.Error()
// is mockConnSetWriteDeadlinePanic.
func IsMockConnSetWriteDeadlinePanic(t testing.TB, err error) {
	require.Contains(t, err.Error(), mockConnSetWriteDeadlinePanic)
}

// IsMockConnClosedError is used to check err is errMockConnClosed.
func IsMockConnClosedError(t testing.TB, err error) {
	require.Equal(t, errMockConnClosed, err)
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

// IsMockListenerAcceptPanic is used to check err.Error()
// is mockListenerAcceptPanic.
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

// mockResponseWriter is used to test for Hijack().
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

func (rw *mockResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if rw.hijack {
		return nil, nil, errors.New("failed to hijack")
	}
	return rw.conn, nil, nil
}

// NewMockResponseWriter is used to create simple mock response writer.
func NewMockResponseWriter() http.ResponseWriter {
	return &mockResponseWriter{conn: NewMockConn()}
}

// NewMockResponseWriterWithHijackError is used to create a mock
// http.ResponseWriter that implemented http.Hijacker, if call Hijack()
// it will return an error.
func NewMockResponseWriterWithHijackError() http.ResponseWriter {
	return &mockResponseWriter{hijack: true}
}

// NewMockResponseWriterWithWriteError is used to create a mock
// http.ResponseWriter that implemented http.Hijacker, if use hijacked
// connection and when call Write(), it will return an error.
func NewMockResponseWriterWithWriteError() http.ResponseWriter {
	return &mockResponseWriter{conn: NewMockConnWithWriteError()}
}

// NewMockResponseWriterWithClosePanic is used to create a mock
// http.ResponseWriter that implemented http.Hijacker, if use hijacked
// connection and when call Close() it will panic.
func NewMockResponseWriterWithClosePanic() http.ResponseWriter {
	return &mockResponseWriter{conn: NewMockConnWithClosePanic()}
}
