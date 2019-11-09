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
	// check
	require.Equal(t, http.MethodPost, request.Method)
	require.Equal(t, "https://127.0.0.1/", request.URL.String())
	postData, err := ioutil.ReadAll(request.Body)
	require.NoError(t, err)
	require.Equal(t, []byte{1, 2}, postData)
	require.Equal(t, []string{"keep-alive"}, request.Header["Connection"])
	require.Equal(t, 7, len(request.Header))
	require.Equal(t, "localhost", request.Host)
	require.Equal(t, true, request.Close)
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
	// check
	const timeout = 10 * time.Second
	require.Equal(t, 2, transport.MaxIdleConns)
	require.Equal(t, 2, transport.MaxIdleConnsPerHost)
	require.Equal(t, 2, transport.MaxConnsPerHost)
	require.Equal(t, timeout, transport.TLSHandshakeTimeout)
	require.Equal(t, timeout, transport.IdleConnTimeout)
	require.Equal(t, timeout, transport.ResponseHeaderTimeout)
	require.Equal(t, timeout, transport.ExpectContinueTimeout)
	require.Equal(t, int64(16384), transport.MaxResponseHeaderBytes)
	require.Equal(t, true, transport.DisableKeepAlives)
	require.Equal(t, true, transport.DisableCompression)
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
	// check
	const timeout = 10 * time.Second
	require.Equal(t, timeout, server.ReadTimeout)
	require.Equal(t, timeout, server.WriteTimeout)
	require.Equal(t, timeout, server.ReadHeaderTimeout)
	require.Equal(t, timeout, server.IdleTimeout)
	require.Equal(t, 16384, server.MaxHeaderBytes)
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
