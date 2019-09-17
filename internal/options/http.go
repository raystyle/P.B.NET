package options

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"time"
)

const (
	defaultTimeout        = time.Minute
	defaultMaxHeaderBytes = 4 * 1048576 // 4MB
)

type HTTPRequest struct {
	Method string      `toml:"method"`
	URL    string      `toml:"url"`
	Post   string      `toml:"post"` // hex
	Header http.Header `toml:"header"`
	Host   string      `toml:"host"`
	Close  bool        `toml:"close"`
}

func (hr *HTTPRequest) failed(err error) error {
	return fmt.Errorf("apply http request failed: %s", err)
}

func (hr *HTTPRequest) Apply() (*http.Request, error) {
	post, err := hex.DecodeString(hr.Post)
	if err != nil {
		return nil, hr.failed(err)
	}
	r, err := http.NewRequest(hr.Method, hr.URL, bytes.NewReader(post))
	if err != nil {
		return nil, hr.failed(err)
	}
	r.Header = hr.Header.Clone()
	r.Host = hr.Host
	r.Close = hr.Close
	return r, nil
}

type HTTPTransport struct {
	TLSClientConfig        TLSConfig     `toml:"tls_client_config"`
	TLSHandshakeTimeout    time.Duration `toml:"tls_handshake_timeout"`
	DisableKeepAlives      bool          `toml:"disable_keep_alives"`
	DisableCompression     bool          `toml:"disable_compression"`
	MaxIdleConns           int           `toml:"max_idle_conns"`
	MaxIdleConnsPerHost    int           `toml:"max_idle_conns_per_host"`
	MaxConnsPerHost        int           `toml:"max_conns_per_host"`
	IdleConnTimeout        time.Duration `toml:"idle_conn_timeout"`
	ResponseHeaderTimeout  time.Duration `toml:"response_header_timeout"`
	ExpectContinueTimeout  time.Duration `toml:"expect_continue_timeout"`
	MaxResponseHeaderBytes int64         `toml:"max_response_header_bytes"`
	ForceAttemptHTTP2      bool          `toml:"force_attempt_http2"`
}

func (ht *HTTPTransport) failed(err error) error {
	return fmt.Errorf("apply http transport failed: %s", err)
}

func (ht *HTTPTransport) Apply() (*http.Transport, error) {
	tr := &http.Transport{
		TLSHandshakeTimeout:    ht.TLSHandshakeTimeout,
		DisableKeepAlives:      ht.DisableKeepAlives,
		DisableCompression:     ht.DisableCompression,
		MaxIdleConns:           ht.MaxIdleConns,
		IdleConnTimeout:        ht.IdleConnTimeout,
		ResponseHeaderTimeout:  ht.ResponseHeaderTimeout,
		ExpectContinueTimeout:  ht.ExpectContinueTimeout,
		MaxResponseHeaderBytes: ht.MaxResponseHeaderBytes,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2: ht.ForceAttemptHTTP2,
	}
	// tls config
	var err error
	tr.TLSClientConfig, err = ht.TLSClientConfig.Apply()
	if err != nil {
		return nil, ht.failed(err)
	}
	// conn
	if ht.MaxIdleConnsPerHost < 1 {
		tr.MaxIdleConnsPerHost = 1
	}
	if ht.MaxConnsPerHost < 1 {
		tr.MaxConnsPerHost = 1
	}
	// timeout
	if ht.TLSHandshakeTimeout < 1 {
		tr.TLSHandshakeTimeout = defaultTimeout
	}
	if ht.IdleConnTimeout < 1 {
		tr.IdleConnTimeout = defaultTimeout
	}
	if ht.ResponseHeaderTimeout < 1 {
		tr.ResponseHeaderTimeout = defaultTimeout
	}
	if ht.ExpectContinueTimeout < 1 {
		tr.ExpectContinueTimeout = defaultTimeout
	}
	// max header bytes
	if ht.MaxResponseHeaderBytes < 1 {
		tr.MaxResponseHeaderBytes = defaultMaxHeaderBytes
	}
	return tr, nil
}

type HTTPServer struct {
	TLSConfig         TLSConfig     `toml:"tls_client_config"`
	ReadTimeout       time.Duration `toml:"read_timeout"`  // warning
	WriteTimeout      time.Duration `toml:"write_timeout"` // warning
	ReadHeaderTimeout time.Duration `toml:"read_header_timeout"`
	IdleTimeout       time.Duration `toml:"idle_timeout"`
	MaxHeaderBytes    int           `toml:"max_header_bytes"`
	DisableKeepAlive  bool          `toml:"disable_keep_alive"`
}

func (hs *HTTPServer) failed(err error) error {
	return fmt.Errorf("apply http server failed: %s", err)
}

func (hs *HTTPServer) Apply() (*http.Server, error) {
	s := &http.Server{
		ReadTimeout:       hs.ReadTimeout,
		WriteTimeout:      hs.WriteTimeout,
		ReadHeaderTimeout: hs.ReadHeaderTimeout,
		IdleTimeout:       hs.IdleTimeout,
		MaxHeaderBytes:    hs.MaxHeaderBytes,
	}
	// tls config
	var err error
	s.TLSConfig, err = hs.TLSConfig.Apply()
	if err != nil {
		return nil, hs.failed(err)
	}
	// timeout
	if hs.ReadHeaderTimeout < 1 {
		s.ReadHeaderTimeout = defaultTimeout
	}
	if hs.IdleTimeout < 1 {
		s.IdleTimeout = defaultTimeout
	}
	// max header bytes
	if hs.MaxHeaderBytes < 1 {
		s.MaxHeaderBytes = defaultMaxHeaderBytes
	}
	s.SetKeepAlivesEnabled(!hs.DisableKeepAlive)
	return s, nil
}
