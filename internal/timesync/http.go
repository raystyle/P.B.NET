package timesync

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/dns"
	"project/internal/options"
	"project/internal/proxy"
	ht "project/internal/timesync/http"
)

var ErrQueryHTTPFailed = errors.New("query http server failed")

type HTTPClient struct {
	proxyPool *proxy.Pool
	dnsClient *dns.Client

	Request   options.HTTPRequest   `toml:"request"`
	Transport options.HTTPTransport `toml:"transport"`
	Timeout   time.Duration         `toml:"timeout"`
	ProxyTag  string                `toml:"proxy_tag"`
	DNSOpts   dns.Options           `toml:"dns_options"`
}

func NewHTTPClient(config []byte) (*HTTPClient, error) {
	hc := HTTPClient{}
	err := toml.Unmarshal(config, &hc)
	if err != nil {
		return nil, err
	}
	return &hc, nil
}

func (client *HTTPClient) Query() (now time.Time, isOptsErr bool, err error) {
	// http request
	req, err := client.Request.Apply()
	if err != nil {
		isOptsErr = true
		return
	}
	hostname := req.URL.Hostname()
	// http transport
	tr, err := client.Transport.Apply()
	if err != nil {
		isOptsErr = true
		return
	}
	tr.TLSClientConfig.ServerName = hostname
	// set proxy
	p, err := client.proxyPool.Get(client.ProxyTag)
	if err != nil {
		isOptsErr = true
		return
	}
	p.HTTP(tr)
	// dns
	ipList, err := client.dnsClient.Resolve(hostname, &client.DNSOpts)
	if err != nil {
		isOptsErr = true
		err = fmt.Errorf("resolve domain name failed: %s", err)
		return
	}
	// https://github.com/ -> http://1.1.1.1:443/
	if req.URL.Scheme == "https" {
		if req.Host == "" {
			req.Host = req.URL.Host
		}
	}
	port := req.URL.Port()
	if port != "" {
		port = ":" + port
	}
	switch client.DNSOpts.Type {
	case "", dns.IPv4:
		for i := 0; i < len(ipList); i++ {
			// replace to ip
			req.URL.Host = ipList[i] + port
			now, err = ht.Query(req, &http.Client{
				Transport: tr,
				Timeout:   client.Timeout,
			})
			if err == nil {
				return
			}
		}
	case dns.IPv6:
		for i := 0; i < len(ipList); i++ {
			// replace to ip
			req.URL.Host = "[" + ipList[i] + "]" + port
			now, err = ht.Query(req, &http.Client{
				Transport: tr,
				Timeout:   client.Timeout,
			})
			if err == nil {
				return
			}
		}
	default:
		err = fmt.Errorf("timesyncer internal error: %s",
			dns.UnknownTypeError(client.DNSOpts.Type))
		panic(err)
	}
	err = ErrQueryHTTPFailed
	return
}

func (client *HTTPClient) ImportConfig(b []byte) error {
	return msgpack.Unmarshal(b, client)
}

func (client *HTTPClient) ExportConfig() []byte {
	b, err := msgpack.Marshal(client)
	if err != nil {
		panic(err)
	}
	return b
}
