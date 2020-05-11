package timesync

import (
	"context"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"project/internal/crypto/cert"
	"project/internal/dns"
	"project/internal/option"
	"project/internal/patch/toml"
	"project/internal/proxy"
	"project/internal/random"
)

const defaultDialTimeout = 30 * time.Second

// HTTP is used to create a HTTP client to do request
// that get date in response header.
type HTTP struct {
	ctx       context.Context
	certPool  *cert.Pool
	proxyPool *proxy.Pool
	dnsClient *dns.Client

	Request   option.HTTPRequest   `toml:"request" check:"-"`
	Transport option.HTTPTransport `toml:"transport" check:"-"`
	Timeout   time.Duration        `toml:"timeout"`
	ProxyTag  string               `toml:"proxy_tag"`
	DNSOpts   dns.Options          `toml:"dns" check:"-"`
}

// NewHTTP is used to create a HTTP client.
func NewHTTP(
	ctx context.Context,
	certPool *cert.Pool,
	proxyPool *proxy.Pool,
	dnsClient *dns.Client,
) *HTTP {
	return &HTTP{
		ctx:       ctx,
		certPool:  certPool,
		proxyPool: proxyPool,
		dnsClient: dnsClient,
	}
}

// Query is used to query time from response header.
func (h *HTTP) Query() (now time.Time, optsErr bool, err error) {
	// http request
	req, err := h.Request.Apply()
	if err != nil {
		optsErr = true
		return
	}
	hostname := req.URL.Hostname()

	// http transport
	h.Transport.TLSClientConfig.CertPool = h.certPool
	tr, err := h.Transport.Apply()
	if err != nil {
		optsErr = true
		return
	}
	if tr.TLSClientConfig.ServerName == "" {
		tr.TLSClientConfig.ServerName = hostname
	}

	// set proxy
	proxyClient, err := h.proxyPool.Get(h.ProxyTag)
	if err != nil {
		optsErr = true
		return
	}
	proxyClient.HTTP(tr)

	// resolve domain name
	result, err := h.dnsClient.ResolveContext(h.ctx, hostname, &h.DNSOpts)
	if err != nil {
		optsErr = true
		err = errors.WithMessage(err, "failed to resolve domain name")
		return
	}

	// do http request
	port := req.URL.Port()
	httpClient := &http.Client{
		Transport: tr,
		Timeout:   h.Timeout,
	}

	for i := 0; i < len(result); i++ {
		req := req.Clone(h.ctx)
		// replace to IP address
		if port != "" {
			req.URL.Host = net.JoinHostPort(result[i], port)
		} else {
			req.URL.Host = result[i]
		}
		// set Host header
		// http://www.msfconnecttest.com/ -> http://96.126.123.244/
		// http will set host that not show domain name
		// but https useless, because of TLS.
		if req.Host == "" && req.URL.Scheme == "http" {
			req.Host = req.URL.Host
		}
		now, err = getHeaderDate(req, httpClient)
		if err == nil {
			break
		}
	}
	if err == nil {
		return
	}
	err = errors.Errorf("failed to query http server: %s", err)
	return
}

// getHeaderDate is used to get date from http response header.
func getHeaderDate(req *http.Request, client *http.Client) (time.Time, error) {
	defer client.CloseIdleConnections()
	if client.Timeout < 1 {
		client.Timeout = defaultDialTimeout
	}
	t1 := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	// TCP: 3 RTT, TLS 4 RTT(most), Request 1 RTT, Response(this) 1 RTT
	rtt := time.Duration(5)
	if req.URL.Scheme == "https" {
		rtt += 4
	}
	delta := time.Since(t1) / rtt
	// <security> prevent system time changed
	if delta > 10*time.Second || delta < 0 {
		delta = 10 * time.Second
	}
	defer func() {
		// <security> read limit
		n := int64(4<<20 + random.Int(4<<20)) // 4-8 MB
		_, _ = io.CopyN(ioutil.Discard, resp.Body, n)
		_ = resp.Body.Close()
	}()
	remoteTime, err := http.ParseTime(resp.Header.Get("Date"))
	if err != nil {
		return time.Time{}, err
	}
	return remoteTime.Add(delta), nil
}

// Import is for time syncer.
func (h *HTTP) Import(b []byte) error {
	return toml.Unmarshal(b, h)
}

// Export is for time syncer.
func (h *HTTP) Export() []byte {
	b, _ := toml.Marshal(h)
	return b
}

// TestHTTP is used to create a HTTP client to test toml config.
func TestHTTP(config []byte) error {
	return new(HTTP).Import(config)
}
