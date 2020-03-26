package httptool

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrintRequest(t *testing.T) {
	r, err := http.NewRequest(http.MethodGet, "https://github.com/", nil)
	require.NoError(t, err)
	r.RemoteAddr = "127.0.0.1:1234"
	r.RequestURI = "/index"
	r.Header.Set("User-Agent", "Mozilla")
	r.Header.Set("Accept", "text/html")
	r.Header.Set("Connection", "keep-alive")

	fmt.Println("-----begin (GET and no body)-----")
	fmt.Println(PrintRequest(r))
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
	fmt.Println(PrintRequest(r))
	fmt.Printf("-----end-----\n\n")
	equalBody(rawBody, r.Body)

	body.Reset()
	body.WriteString(strings.Repeat("a", bodyLineLength-10))
	rawBody = bytes.NewReader(body.Bytes())
	r.Body = ioutil.NopCloser(body)
	fmt.Println("-----begin (POST with data <bodyLineLength)-----")
	fmt.Println(PrintRequest(r))
	fmt.Printf("-----end-----\n\n")
	equalBody(rawBody, r.Body)

	body.Reset()
	body.WriteString(strings.Repeat("a", bodyLineLength))
	rawBody = bytes.NewReader(body.Bytes())
	r.Body = ioutil.NopCloser(body)
	fmt.Println("-----begin (POST with data bodyLineLength)-----")
	fmt.Println(PrintRequest(r))
	fmt.Printf("-----end-----\n\n")
	equalBody(rawBody, r.Body)

	body.Reset()
	body.WriteString(strings.Repeat("a", 3*bodyLineLength-1))
	rawBody = bytes.NewReader(body.Bytes())
	r.Body = ioutil.NopCloser(body)
	fmt.Println("-----begin (POST with data 3*bodyLineLength-1)-----")
	fmt.Println(PrintRequest(r))
	fmt.Printf("-----end-----\n\n")
	equalBody(rawBody, r.Body)

	body.Reset()
	body.WriteString(strings.Repeat("a", 100*bodyLineLength-1))
	rawBody = bytes.NewReader(body.Bytes())
	r.Body = ioutil.NopCloser(body)
	fmt.Println("-----begin (POST with data 100*bodyLineLength-1)-----")
	fmt.Println(PrintRequest(r))
	fmt.Printf("-----end-----\n\n")
	equalBody(rawBody, r.Body)
}
