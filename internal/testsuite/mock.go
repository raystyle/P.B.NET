package testsuite

import (
	"errors"
	"net"
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

type mockListenerWithError struct{}

func (ml mockListenerWithError) Accept() (net.Conn, error) {
	return nil, ErrMockListener
}

func (mockListenerWithError) Close() error {
	return nil
}

func (mockListenerWithError) Addr() net.Addr {
	return new(mockListenerAddr)
}

// NewMockListenerWithError is used to create a mock listener
// that return a custom error call Accept()
func NewMockListenerWithError() net.Listener {
	return new(mockListenerWithError)
}

// IsMockListenerError is used to confirm err is ErrMockListener
func IsMockListenerError(t testing.TB, err error) {
	require.Equal(t, ErrMockListener, err)
}

type mockListenerWithPanic struct{}

func (ml mockListenerWithPanic) Accept() (net.Conn, error) {
	defer func() { panic(MockListenerPanic) }()
	return nil, nil
}

func (mockListenerWithPanic) Close() error {
	return nil
}

func (mockListenerWithPanic) Addr() net.Addr {
	return new(mockListenerAddr)
}

// NewMockListenerWithPanic is used to create a mock listener
// that panic when call Accept()
func NewMockListenerWithPanic() net.Listener {
	return new(mockListenerWithPanic)
}

// IsMockListenerPanic is used to confirm err.Error() is MockListenerPanic
func IsMockListenerPanic(t testing.TB, err error) {
	require.Contains(t, err.Error(), MockListenerPanic)
}
