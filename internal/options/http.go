package options

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
)

const (
	default_timeout        = time.Minute
	default_MaxHeaderBytes = 4 * 1048576 // 4MB
)

type HTTP_Request struct {
	Method string
	URL    string
	Post   string // hex
	Header http.Header
	Host   string
	Close  bool
}

func (this *HTTP_Request) failed(err error) error {
	return fmt.Errorf("http request apply failed: %s", err)
}

func (this *HTTP_Request) Apply() (*http.Request, error) {
	post, err := hex.DecodeString(this.Post)
	if err != nil {
		return nil, this.failed(err)
	}
	r, err := http.NewRequest(this.Method, this.URL, bytes.NewReader(post))
	if err != nil {
		return nil, this.failed(err)
	}
	r.Header = Copy_HTTP_Header(this.Header)
	r.Host = this.Host
	r.Close = this.Close
	return r, nil
}

type HTTP_Transport struct {
	TLSClientConfig        TLS_Config
	TLSHandshakeTimeout    time.Duration
	DisableKeepAlives      bool
	DisableCompression     bool
	MaxIdleConns           int
	MaxIdleConnsPerHost    int
	MaxConnsPerHost        int
	IdleConnTimeout        time.Duration
	ResponseHeaderTimeout  time.Duration
	ExpectContinueTimeout  time.Duration
	MaxResponseHeaderBytes int64
}

func (this *HTTP_Transport) failed(err error) error {
	return fmt.Errorf("http transport apply failed: %s", err)
}

func (this *HTTP_Transport) Apply() (*http.Transport, error) {
	tr := &http.Transport{
		TLSHandshakeTimeout:    this.TLSHandshakeTimeout,
		DisableKeepAlives:      this.DisableKeepAlives,
		DisableCompression:     this.DisableCompression,
		MaxIdleConns:           this.MaxIdleConns,
		IdleConnTimeout:        this.IdleConnTimeout,
		ResponseHeaderTimeout:  this.ResponseHeaderTimeout,
		ExpectContinueTimeout:  this.ExpectContinueTimeout,
		MaxResponseHeaderBytes: this.MaxResponseHeaderBytes,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	// tls config
	var err error
	tr.TLSClientConfig, err = this.TLSClientConfig.Apply()
	if err != nil {
		return nil, this.failed(err)
	}
	// config http/2
	err = http2.ConfigureTransport(tr)
	if err != nil {
		return nil, this.failed(err)
	}
	// conn
	if this.MaxIdleConnsPerHost < 1 {
		tr.MaxIdleConnsPerHost = 1
	}
	if this.MaxConnsPerHost < 1 {
		tr.MaxConnsPerHost = 1
	}
	// timeout
	if this.TLSHandshakeTimeout < 1 {
		tr.TLSHandshakeTimeout = default_timeout
	}
	if this.IdleConnTimeout < 1 {
		tr.IdleConnTimeout = default_timeout
	}
	if this.ResponseHeaderTimeout < 1 {
		tr.ResponseHeaderTimeout = default_timeout
	}
	if this.ExpectContinueTimeout < 1 {
		tr.ExpectContinueTimeout = default_timeout
	}
	// max header bytes
	if this.MaxResponseHeaderBytes < 1 {
		tr.MaxResponseHeaderBytes = default_MaxHeaderBytes
	}
	return tr, nil
}

type HTTP_Server struct {
	TLSConfig         TLS_Config
	ReadTimeout       time.Duration // warning
	WriteTimeout      time.Duration // warning
	ReadHeaderTimeout time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
	DisableKeepAlive  bool
}

func (this *HTTP_Server) failed(err error) error {
	return fmt.Errorf("http server apply failed: %s", err)
}

func (this *HTTP_Server) Apply() (*http.Server, error) {
	s := &http.Server{
		ReadTimeout:       this.ReadTimeout,
		WriteTimeout:      this.WriteTimeout,
		ReadHeaderTimeout: this.ReadHeaderTimeout,
		IdleTimeout:       this.IdleTimeout,
		MaxHeaderBytes:    this.MaxHeaderBytes,
	}
	// tls config
	var err error
	s.TLSConfig, err = this.TLSConfig.Apply()
	if err != nil {
		return nil, this.failed(err)
	}
	// timeout
	if this.ReadHeaderTimeout < 1 {
		s.ReadHeaderTimeout = default_timeout
	}
	if this.IdleTimeout < 1 {
		s.IdleTimeout = default_timeout
	}
	// max header bytes
	if this.MaxHeaderBytes < 1 {
		s.MaxHeaderBytes = default_MaxHeaderBytes
	}
	s.SetKeepAlivesEnabled(!this.DisableKeepAlive)
	return s, nil
}

// from GOROOT/src/cmd/go/internal/web2/web.go
func Copy_HTTP_Header(hdr http.Header) http.Header {
	h2 := make(http.Header)
	if hdr == nil {
		return h2
	}
	for k, v := range hdr {
		v2 := make([]string, len(v))
		copy(v2, v)
		h2[k] = v2
	}
	return h2
}
