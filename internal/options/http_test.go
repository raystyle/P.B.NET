package options

import (
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"
)

func TestHTTPRequestDefault(t *testing.T) {
	const url = "http://127.0.0.1/"
	hr := &HTTPRequest{URL: url}
	request, err := hr.Apply()
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, request.Method)
	require.Equal(t, url, request.URL.String())
	require.Equal(t, http.NoBody, request.Body)
	require.NotNil(t, request.Header)
	require.Equal(t, "", request.Host)
	require.Equal(t, false, request.Close)
}

func TestHTTPRequestUnmarshal(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/http_request.toml")
	require.NoError(t, err)
	hr := HTTPRequest{}
	err = toml.Unmarshal(data, &hr)
	require.NoError(t, err)
	request, err := hr.Apply()
	require.NoError(t, err)
	postData, err := ioutil.ReadAll(request.Body)
	require.NoError(t, err)

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: http.MethodPost, actual: request.Method},
		{expected: "https://127.0.0.1/", actual: request.URL.String()},
		{expected: []byte{1, 2}, actual: postData},
		{expected: []string{"keep-alive"}, actual: request.Header["Connection"]},
		{expected: 7, actual: len(request.Header)},
		{expected: "localhost", actual: request.Host},
		{expected: true, actual: request.Close},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}
}

func TestHTTPRequest_Apply_failed(t *testing.T) {
	// empty url
	hr := HTTPRequest{}
	_, err := hr.Apply()
	require.Errorf(t, err, "failed to apply http request options: empty url")

	// invalid post data
	hr = HTTPRequest{
		URL:  "http://localhost/",
		Post: "foo post data",
	}
	_, err = hr.Apply()
	require.Error(t, err)

	// invalid method
	hr.Post = "0102"
	hr.Method = "invalid method"
	_, err = hr.Apply()
	require.Error(t, err)
}

var testInvalidTLSConfig = TLSConfig{
	RootCAs: []string{"foo data"},
}

func TestHTTPTransportDefault(t *testing.T) {
	transport, err := new(HTTPTransport).Apply()
	require.NoError(t, err)
	require.Equal(t, 1, transport.MaxIdleConns)
	require.Equal(t, 1, transport.MaxIdleConnsPerHost)
	require.Equal(t, 1, transport.MaxConnsPerHost)
	require.Equal(t, httpDefaultTimeout, transport.TLSHandshakeTimeout)
	require.Equal(t, httpDefaultTimeout, transport.IdleConnTimeout)
	require.Equal(t, httpDefaultTimeout, transport.ResponseHeaderTimeout)
	require.Equal(t, httpDefaultTimeout, transport.ExpectContinueTimeout)
	require.Equal(t, httpDefaultMaxResponseHeaderBytes, transport.MaxResponseHeaderBytes)
	require.Equal(t, false, transport.DisableKeepAlives)
	require.Equal(t, false, transport.DisableCompression)
}

func TestHTTPTransportUnmarshal(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/http_transport.toml")
	require.NoError(t, err)
	ht := HTTPTransport{}
	err = toml.Unmarshal(data, &ht)
	require.NoError(t, err)
	transport, err := ht.Apply()
	require.NoError(t, err)
	const timeout = 10 * time.Second

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: 2, actual: transport.MaxIdleConns},
		{expected: 2, actual: transport.MaxIdleConnsPerHost},
		{expected: 2, actual: transport.MaxConnsPerHost},
		{expected: timeout, actual: transport.TLSHandshakeTimeout},
		{expected: timeout, actual: transport.IdleConnTimeout},
		{expected: timeout, actual: transport.ResponseHeaderTimeout},
		{expected: timeout, actual: transport.ExpectContinueTimeout},
		{expected: int64(16384), actual: transport.MaxResponseHeaderBytes},
		{expected: true, actual: transport.DisableKeepAlives},
		{expected: true, actual: transport.DisableCompression},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}
}

func TestHTTPTransport_Apply_failed(t *testing.T) {
	// invalid tls config
	tr := HTTPTransport{
		TLSClientConfig: testInvalidTLSConfig,
	}
	_, err := tr.Apply()
	require.Error(t, err)
}

func TestHTTPServerDefault(t *testing.T) {
	server, err := new(HTTPServer).Apply()
	require.NoError(t, err)
	require.Equal(t, time.Duration(0), server.ReadTimeout)
	require.Equal(t, time.Duration(0), server.WriteTimeout)
	require.Equal(t, httpDefaultTimeout, server.ReadHeaderTimeout)
	require.Equal(t, httpDefaultTimeout, server.IdleTimeout)
	require.Equal(t, httpDefaultMaxHeaderBytes, server.MaxHeaderBytes)
}

func TestHTTPServerUnmarshal(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/http_server.toml")
	require.NoError(t, err)
	hs := HTTPServer{}
	err = toml.Unmarshal(data, &hs)
	require.NoError(t, err)
	server, err := hs.Apply()
	require.NoError(t, err)
	const timeout = 10 * time.Second

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: timeout, actual: server.ReadTimeout},
		{expected: timeout, actual: server.WriteTimeout},
		{expected: timeout, actual: server.ReadHeaderTimeout},
		{expected: timeout, actual: server.IdleTimeout},
		{expected: 16384, actual: server.MaxHeaderBytes},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}
}

func TestHTTPServer_Apply_failed(t *testing.T) {
	// invalid tls config
	s := HTTPServer{
		TLSConfig: testInvalidTLSConfig,
	}
	_, err := s.Apply()
	require.Error(t, err)
}

func TestHTTPServerSetReadWriteTimeout(t *testing.T) {
	// invalid tls config
	s := HTTPServer{
		ReadTimeout:  -1,
		WriteTimeout: -1,
	}
	_, err := s.Apply()
	require.NoError(t, err)
}
