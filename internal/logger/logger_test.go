package logger

import (
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testSrc  = "test src"
	testLog1 = "test"
	testLog2 = "log"
)

func TestLogger(t *testing.T) {
	Test.Printf(Debug, testSrc, "test-format %s %s", testLog1, testLog2)
	Test.Print(Debug, testSrc, "test-print", testLog1, testLog2)
	Test.Println(Debug, testSrc, "test-println", testLog1, testLog2)
	Discard.Printf(Debug, testSrc, "test-format %s %s", testLog1, testLog2)
	Discard.Print(Debug, testSrc, "test-print", testLog1, testLog2)
	Discard.Println(Debug, testSrc, "test-println", testLog1, testLog2)
}

func TestParse(t *testing.T) {
	l, err := Parse("debug")
	require.NoError(t, err)
	require.Equal(t, l, Debug)
	l, err = Parse("info")
	require.NoError(t, err)
	require.Equal(t, l, Info)
	l, err = Parse("warning")
	require.NoError(t, err)
	require.Equal(t, l, Warning)
	l, err = Parse("error")
	require.NoError(t, err)
	require.Equal(t, l, Error)
	l, err = Parse("exploit")
	require.NoError(t, err)
	require.Equal(t, l, Exploit)
	l, err = Parse("fatal")
	require.NoError(t, err)
	require.Equal(t, l, Fatal)
	l, err = Parse("off")
	require.NoError(t, err)
	require.Equal(t, l, Off)
	l, err = Parse("invalid level")
	require.Error(t, err)
	require.Equal(t, l, Debug)
}

func TestPrefix(t *testing.T) {
	for l := Level(0); l < Off; l++ {
		t.Log(Prefix(l, testSrc).String())
	}
	// unknown level
	t.Log(Prefix(Level(153), testSrc).String())
}

func TestWrap(t *testing.T) {
	l := Wrap(Debug, "test wrap", Test)
	l.Println("println")
}

func TestConn(t *testing.T) {
	conn, err := net.Dial("tcp", "github.com:443")
	require.NoError(t, err)
	t.Log(Conn(conn))
	_ = conn.Close()
}

func TestHTTPRequest(t *testing.T) {
	r := http.Request{
		Method:     http.MethodGet,
		RequestURI: "/",
		RemoteAddr: "127.0.0.1:1234",
	}
	t.Log(HTTPRequest(&r))
}
