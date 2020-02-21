package option

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	httpDefaultTimeout                      = time.Minute
	httpDefaultMaxHeaderBytes               = 512 * 1024 // 512KB
	httpDefaultMaxResponseHeaderBytes int64 = 512 * 1024 // 512KB
)

// HTTPRequest include options about http.Request.
type HTTPRequest struct {
	Method string      `toml:"method"`
	URL    string      `toml:"url"`
	Post   string      `toml:"post"` // hex
	Header http.Header `toml:"header"`
	Host   string      `toml:"host"`
	Close  bool        `toml:"close"`
}

func (hr *HTTPRequest) error(err error) error {
	return fmt.Errorf("failed to apply http request options: %s", err)
}

// Apply is used to create *http.Request.
func (hr *HTTPRequest) Apply() (*http.Request, error) {
	if hr.URL == "" {
		return nil, hr.error(errors.New("empty url"))
	}
	post, err := hex.DecodeString(hr.Post)
	if err != nil {
		return nil, hr.error(err)
	}
	r, err := http.NewRequest(hr.Method, hr.URL, bytes.NewReader(post))
	if err != nil {
		return nil, hr.error(err)
	}
	r.Header = hr.Header.Clone()
	if r.Header == nil {
		r.Header = make(http.Header)
	}
	r.Host = hr.Host
	r.Close = hr.Close
	return r, nil
}

// HTTPTransport include options about http.Transport.
type HTTPTransport struct {
	TLSClientConfig        TLSConfig     `toml:"tls_config"`
	MaxIdleConns           int           `toml:"max_idle_conns"`
	MaxIdleConnsPerHost    int           `toml:"max_idle_conns_per_host"`
	MaxConnsPerHost        int           `toml:"max_conns_per_host"`
	TLSHandshakeTimeout    time.Duration `toml:"tls_handshake_timeout"`
	IdleConnTimeout        time.Duration `toml:"idle_conn_timeout"`
	ResponseHeaderTimeout  time.Duration `toml:"response_header_timeout"`
	ExpectContinueTimeout  time.Duration `toml:"expect_continue_timeout"`
	MaxResponseHeaderBytes int64         `toml:"max_response_header_bytes"`
	DisableKeepAlives      bool          `toml:"disable_keep_alives"`
	DisableCompression     bool          `toml:"disable_compression"`
}

// Apply is used to create *http.Transport.
//
// when set MaxConnsPerHost, use HTTP/2 get test.com will panic, wait golang fix it.
func (ht *HTTPTransport) Apply() (*http.Transport, error) {
	tr := &http.Transport{
		MaxIdleConns:        ht.MaxIdleConns,
		MaxIdleConnsPerHost: ht.MaxIdleConnsPerHost,
		// MaxConnsPerHost:        ht.MaxConnsPerHost,
		TLSHandshakeTimeout:    ht.TLSHandshakeTimeout,
		IdleConnTimeout:        ht.IdleConnTimeout,
		ResponseHeaderTimeout:  ht.ResponseHeaderTimeout,
		ExpectContinueTimeout:  ht.ExpectContinueTimeout,
		MaxResponseHeaderBytes: ht.MaxResponseHeaderBytes,
		DisableKeepAlives:      ht.DisableKeepAlives,
		DisableCompression:     ht.DisableCompression,
	}
	// tls config
	var err error
	tr.TLSClientConfig, err = ht.TLSClientConfig.Apply()
	if err != nil {
		return nil, err
	}
	// conn
	if tr.MaxIdleConns < 1 {
		tr.MaxIdleConns = 1
	}
	if tr.MaxIdleConnsPerHost < 1 {
		tr.MaxIdleConnsPerHost = 1
	}
	// if tr.MaxConnsPerHost < 1 {
	// 	tr.MaxConnsPerHost = 1
	// }
	// timeout
	if tr.TLSHandshakeTimeout < 1 {
		tr.TLSHandshakeTimeout = httpDefaultTimeout
	}
	if tr.IdleConnTimeout < 1 {
		tr.IdleConnTimeout = httpDefaultTimeout
	}
	if tr.ResponseHeaderTimeout < 1 {
		tr.ResponseHeaderTimeout = httpDefaultTimeout
	}
	if tr.ExpectContinueTimeout < 1 {
		tr.ExpectContinueTimeout = httpDefaultTimeout
	}
	// max header bytes
	if tr.MaxResponseHeaderBytes < 1 {
		tr.MaxResponseHeaderBytes = httpDefaultMaxResponseHeaderBytes
	}
	return tr, nil
}

// HTTPServer include options about http.Server.
type HTTPServer struct {
	TLSConfig         TLSConfig     `toml:"tls_config"`
	ReadTimeout       time.Duration `toml:"read_timeout"`  // warning
	WriteTimeout      time.Duration `toml:"write_timeout"` // warning
	ReadHeaderTimeout time.Duration `toml:"read_header_timeout"`
	IdleTimeout       time.Duration `toml:"idle_timeout"`
	MaxHeaderBytes    int           `toml:"max_header_bytes"`
	DisableKeepAlive  bool          `toml:"disable_keep_alive"`
}

// Apply is used to create *http.Server.
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
		return nil, err
	}
	// timeout
	if s.ReadTimeout < 0 {
		s.ReadTimeout = httpDefaultTimeout
	}
	if s.WriteTimeout < 0 {
		s.WriteTimeout = httpDefaultTimeout
	}
	if s.ReadHeaderTimeout < 1 {
		s.ReadHeaderTimeout = httpDefaultTimeout
	}
	if s.IdleTimeout < 1 {
		s.IdleTimeout = httpDefaultTimeout
	}
	// max header bytes
	if s.MaxHeaderBytes < 1 {
		s.MaxHeaderBytes = httpDefaultMaxHeaderBytes
	}
	s.SetKeepAlivesEnabled(!hs.DisableKeepAlive)
	return s, nil
}
