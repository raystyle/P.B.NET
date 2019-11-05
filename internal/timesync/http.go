package timesync

import (
	"context"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	"project/internal/dns"
	"project/internal/options"
	"project/internal/proxy"
	"project/internal/random"
)

type HTTP struct {
	// copy from Syncer
	proxyPool *proxy.Pool
	dnsClient *dns.Client
	ctx       context.Context

	Request   options.HTTPRequest   `toml:"request"`
	Transport options.HTTPTransport `toml:"transport"`
	Timeout   time.Duration         `toml:"timeout"`
	ProxyTag  string                `toml:"proxy_tag"`
	DNSOpts   dns.Options           `toml:"dns_options"`
}

// NewHTTP is used to create HTTP
func NewHTTP(ctx context.Context, pool *proxy.Pool, client *dns.Client) *HTTP {
	return &HTTP{
		ctx:       ctx,
		proxyPool: pool,
		dnsClient: client,
	}
}

// Query is used to query time
func (h *HTTP) Query() (now time.Time, optsErr bool, err error) {
	// http request
	req, err := h.Request.Apply()
	if err != nil {
		optsErr = true
		return
	}
	hostname := req.URL.Hostname()

	// http transport
	tr, err := h.Transport.Apply()
	if err != nil {
		optsErr = true
		return
	}
	if tr.TLSClientConfig.ServerName == "" {
		tr.TLSClientConfig.ServerName = hostname
	}

	// set proxy
	p, err := h.proxyPool.Get(h.ProxyTag)
	if err != nil {
		optsErr = true
		return
	}
	p.HTTP(tr)

	// resolve domain name
	dnsOptsCopy := h.DNSOpts
	result, err := h.dnsClient.Resolve(hostname, &dnsOptsCopy)
	if err != nil {
		optsErr = true
		err = errors.WithMessage(err, "failed to resolve domain name")
		return
	}

	// do http request
	port := req.URL.Port()
	hc := &http.Client{
		Transport: tr,
		Timeout:   h.Timeout,
	}

	for i := 0; i < len(result); i++ {
		req := req.Clone(h.ctx)
		// replace to ip
		if port != "" {
			req.URL.Host = net.JoinHostPort(result[i], port)
		} else {
			req.URL.Host = result[i]
		}

		// set Host header
		// http://www.msfconnecttest.com/ -> http://96.126.123.244/
		// http will set host that not show domain name
		// but https useless, because TLS
		if req.Host == "" && req.URL.Scheme == "http" {
			req.Host = req.URL.Host
		}

		now, err = getHeaderDate(req, hc)
		if err == nil {
			return
		}
	}
	err = errors.New("failed to query http server")
	return
}

// getHeaderDate is used to get date from http response header
func getHeaderDate(req *http.Request, client *http.Client) (time.Time, error) {
	defer client.CloseIdleConnections()
	if client.Timeout < 1 {
		client.Timeout = options.DefaultDialTimeout
	}
	resp, err := client.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer func() {
		// <security>
		n := int64(4096 + random.Int(64*(1<<10))) // 4KB - 68KB
		_, _ = io.CopyN(ioutil.Discard, resp.Body, n)
		_ = resp.Body.Close()
	}()
	return http.ParseTime(resp.Header.Get("Date"))
}

// ImportConfig is for time syncer
func (h *HTTP) Import(b []byte) error {
	return toml.Unmarshal(b, h)
}

// ExportConfig is for time syncer
func (h *HTTP) Export() []byte {
	b, err := toml.Marshal(h)
	if err != nil {
		panic(err)
	}
	return b
}

// TestHTTP is used to create a HTTP client to test toml config
func TestHTTP(config []byte) error {
	return new(HTTP).Import(config)
}
