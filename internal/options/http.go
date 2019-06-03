package options

import (
	"bytes"
	"net"
	"net/http"
	"time"
)

const (
	default_timeout        = time.Minute
	default_MaxHeaderBytes = 4 * 1048576 // 4MB
)

type HTTP_Request struct {
	Method string
	URL    string
	Post   []byte
	Header http.Header
}

func (this *HTTP_Request) Apply() (*http.Request, error) {
	r, err := http.NewRequest(this.Method, this.URL, bytes.NewReader(this.Post))
	if err != nil {
		return nil, err
	}
	r.Header = Copy_HTTP_Header(this.Header)
	return r, nil
}

type HTTP_Transport struct {
	TLSClientConfig        *TLS_Config
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

func (this *HTTP_Transport) Apply() (*http.Transport, error) {
	tr := &http.Transport{
		TLSHandshakeTimeout:    default_timeout,
		DisableKeepAlives:      this.DisableKeepAlives,
		DisableCompression:     this.DisableCompression,
		MaxIdleConns:           16,
		MaxIdleConnsPerHost:    4,
		MaxConnsPerHost:        4,
		IdleConnTimeout:        default_timeout,
		ResponseHeaderTimeout:  default_timeout,
		ExpectContinueTimeout:  1 * time.Second,
		MaxResponseHeaderBytes: default_MaxHeaderBytes,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	// tls config
	if this.TLSClientConfig != nil {
		var err error
		tr.TLSClientConfig, err = this.TLSClientConfig.Apply()
		if err != nil {
			return nil, err
		}
	} else {
		tr.TLSClientConfig, _ = new(TLS_Config).Apply()
	}
	// timeout
	if this.TLSHandshakeTimeout > 0 {
		tr.TLSHandshakeTimeout = this.TLSHandshakeTimeout
	}
	if this.IdleConnTimeout > 0 {
		tr.IdleConnTimeout = this.IdleConnTimeout
	}
	if this.ResponseHeaderTimeout > 0 {
		tr.ResponseHeaderTimeout = this.ResponseHeaderTimeout
	}
	if this.ExpectContinueTimeout > 0 {
		tr.ExpectContinueTimeout = this.ExpectContinueTimeout
	}
	// conn
	if this.MaxIdleConns > 0 {
		tr.MaxIdleConns = this.MaxIdleConns
	}
	if this.MaxIdleConnsPerHost > 0 {
		tr.MaxIdleConnsPerHost = this.MaxIdleConnsPerHost
	}
	if this.MaxConnsPerHost > 0 {
		tr.MaxConnsPerHost = this.MaxConnsPerHost
	}
	// max header bytes
	if this.MaxResponseHeaderBytes > 0 {
		tr.MaxResponseHeaderBytes = this.MaxResponseHeaderBytes
	}
	return tr, nil
}

type HTTP_Server struct {
	TLSConfig         *TLS_Config
	ReadTimeout       time.Duration // warning
	WriteTimeout      time.Duration // warning
	ReadHeaderTimeout time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
	DisableKeepAlive  bool
}

func (this *HTTP_Server) Apply() (*http.Server, error) {
	s := &http.Server{
		ReadTimeout:       this.ReadTimeout,
		WriteTimeout:      this.WriteTimeout,
		ReadHeaderTimeout: default_timeout,
		IdleTimeout:       default_timeout,
		MaxHeaderBytes:    default_MaxHeaderBytes,
	}
	// tls config
	if this.TLSConfig != nil {
		var err error
		s.TLSConfig, err = this.TLSConfig.Apply()
		if err != nil {
			return nil, err
		}
	} else {
		s.TLSConfig, _ = new(TLS_Config).Apply()
	}
	// timeout
	if this.ReadHeaderTimeout > 0 {
		s.ReadHeaderTimeout = this.ReadHeaderTimeout
	}
	if this.IdleTimeout > 0 {
		s.IdleTimeout = this.IdleTimeout
	}
	// max header bytes
	if this.MaxHeaderBytes > 0 {
		s.MaxHeaderBytes = this.MaxHeaderBytes
	}
	s.SetKeepAlivesEnabled(!this.DisableKeepAlive)
	return s, nil
}

// from GOROOT/src/cmd/go/internal/web2/web.go
func Copy_HTTP_Header(hdr http.Header) http.Header {
	if hdr == nil {
		return nil
	}
	h2 := make(http.Header)
	for k, v := range hdr {
		v2 := make([]string, len(v))
		copy(v2, v)
		h2[k] = v2
	}
	return h2
}
