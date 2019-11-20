package logger

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
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
	conn, err := net.Dial("tcp", "ds.vm0.test-ipv6.com:80")
	require.NoError(t, err)
	t.Log(Conn(conn))
	_ = conn.Close()
}

func TestHTTPRequest(t *testing.T) {
	r, err := http.NewRequest(http.MethodGet, "https://github.com/", nil)
	require.NoError(t, err)
	r.RemoteAddr = "127.0.0.1:1234"
	r.RequestURI = "/index"
	r.Header.Set("User-Agent", "Mozilla")
	r.Header.Set("Accept", "text/html")
	r.Header.Set("Connection", "keep-alive")

	fmt.Println("-----begin (GET and no body)-----")
	fmt.Println(HTTPRequest(r))
	fmt.Printf("-----end-----\n\n")

	equalBody := func(b1, b2 io.Reader) {
		d1, err := ioutil.ReadAll(b1)
		require.NoError(t, err)
		d2, err := ioutil.ReadAll(b2)
		require.NoError(t, err)
		require.Equal(t, d1, d2)
	}

	body := new(bytes.Buffer)
	rawBody := bytes.NewReader(body.Bytes())
	r.Body = ioutil.NopCloser(body)
	fmt.Println("-----begin (GET with body but no data)-----")
	fmt.Println(HTTPRequest(r))
	fmt.Printf("-----end-----\n\n")
	equalBody(rawBody, r.Body)

	body.Reset()
	body.WriteString(strings.Repeat("a", bodyLineLength-10))
	rawBody = bytes.NewReader(body.Bytes())
	r.Body = ioutil.NopCloser(body)
	fmt.Println("-----begin (POST with data <bodyLineLength)-----")
	fmt.Println(HTTPRequest(r))
	fmt.Printf("-----end-----\n\n")
	equalBody(rawBody, r.Body)

	body.Reset()
	body.WriteString(strings.Repeat("a", bodyLineLength))
	rawBody = bytes.NewReader(body.Bytes())
	r.Body = ioutil.NopCloser(body)
	fmt.Println("-----begin (POST with data bodyLineLength)-----")
	fmt.Println(HTTPRequest(r))
	fmt.Printf("-----end-----\n\n")
	equalBody(rawBody, r.Body)

	body.Reset()
	body.WriteString(strings.Repeat("a", 3*bodyLineLength-1))
	rawBody = bytes.NewReader(body.Bytes())
	r.Body = ioutil.NopCloser(body)
	fmt.Println("-----begin (POST with data 3*bodyLineLength-1)-----")
	fmt.Println(HTTPRequest(r))
	fmt.Printf("-----end-----\n\n")
	equalBody(rawBody, r.Body)

	body.Reset()
	body.WriteString(strings.Repeat("a", 100*bodyLineLength-1))
	rawBody = bytes.NewReader(body.Bytes())
	r.Body = ioutil.NopCloser(body)
	fmt.Println("-----begin (POST with data 100*bodyLineLength-1)-----")
	fmt.Println(HTTPRequest(r))
	fmt.Printf("-----end-----\n\n")
	equalBody(rawBody, r.Body)
}
