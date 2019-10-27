package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"

	"project/internal/crypto/cert"
	"project/internal/options"
)

// Client implement internal/proxy.Client
type Client struct {
	network   string
	address   string
	https     bool
	header    http.Header
	tlsConfig *tls.Config
	timeout   time.Duration

	scheme     string
	rootCAs    []*x509.Certificate
	rootCAsLen int
	proxy      func(*http.Request) (*url.URL, error)
	basicAuth  string
	info       string
}

func NewClient(network, address string, opts *Options) (*Client, error) {
	// check network
	switch network {
	case "", "tcp", "tcp4", "tcp6":
	default:
		return nil, errors.Errorf("unsupport network: %s", network)
	}

	if opts == nil {
		opts = new(Options)
	}

	client := Client{
		network: network,
		address: address,
		https:   opts.HTTPS,
		header:  opts.Header.Clone(),
		timeout: opts.Timeout,
	}

	if client.header == nil {
		client.header = make(http.Header)
	}

	if client.https {
		var err error
		client.tlsConfig, err = opts.TLSConfig.Apply()
		if err != nil {
			return nil, errors.WithStack(err)
		}

		// add system cert and self cert(usually is https proxy server)
		if client.tlsConfig.RootCAs != nil {
			pool, err := cert.SystemCertPool()
			if err != nil {
				return nil, err
			}
			client.rootCAs, _ = opts.TLSConfig.RootCA()
			client.rootCAsLen = len(client.rootCAs)
			for i := 0; i < client.rootCAsLen; i++ {
				if client.rootCAs[i] != nil { // <security>
					pool.AddCert(client.rootCAs[i])
				}
			}
			client.tlsConfig.RootCAs = pool
		}

		if client.tlsConfig.ServerName == "" {
			colonPos := strings.LastIndex(address, ":")
			if colonPos == -1 {
				colonPos = len(address)
			}
			hostname := address[:colonPos]
			c := client.tlsConfig.Clone()
			c.ServerName = hostname
			client.tlsConfig = c
		}
	}

	if client.timeout < 1 {
		client.timeout = options.DefaultDialTimeout
	}

	// set proxy function for Client.HTTP()
	u := &url.URL{Host: address}
	if client.https {
		u.Scheme = "https"
	} else {
		u.Scheme = "http"
	}
	client.scheme = u.Scheme

	// basic authentication
	var auth string
	if opts.Username != "" && opts.Password != "" {
		u.User = url.UserPassword(opts.Username, opts.Password)
		auth = u.User.String()
	} else if opts.Username != "" {
		u.User = url.User(opts.Username)
		auth = u.User.String()
	}
	if auth != "" {
		client.basicAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	}

	// check proxy url
	var err error
	u, err = url.Parse(u.String())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	client.proxy = http.ProxyURL(u)
	client.info = u.String()
	return &client, nil
}

func (c *Client) Dial(_, address string) (net.Conn, error) {
	conn, err := (&net.Dialer{Timeout: c.timeout}).Dial(c.network, c.address)
	if err != nil {
		const format = "dial: failed to connect %s proxy %s"
		return nil, errors.Wrapf(err, format, c.scheme, c.address)
	}
	if c.https {
		conn = tls.Client(conn, c.tlsConfig)
	}
	err = c.Connect(conn, "", address)
	if err != nil {
		_ = conn.Close()
		const format = "dial: %s proxy %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, c.scheme, c.address, address)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func (c *Client) DialContext(ctx context.Context, _, address string) (net.Conn, error) {
	conn, err := (&net.Dialer{Timeout: c.timeout}).DialContext(ctx, c.network, c.address)
	if err != nil {
		const format = "dial context: failed to connect %s proxy %s"
		return nil, errors.Wrapf(err, format, c.scheme, c.address)
	}
	if c.https {
		conn = tls.Client(conn, c.tlsConfig)
	}
	err = c.Connect(conn, "", address)
	if err != nil {
		_ = conn.Close()
		const format = "dial context: %s proxy %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, c.scheme, c.address, address)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func (c *Client) DialTimeout(_, address string, timeout time.Duration) (net.Conn, error) {
	if timeout < 1 {
		timeout = options.DefaultDialTimeout
	}
	conn, err := (&net.Dialer{Timeout: timeout}).Dial(c.network, c.address)
	if err != nil {
		const format = "dial timeout: failed to connect %s proxy %s"
		return nil, errors.Wrapf(err, format, c.scheme, c.address)
	}
	if c.https {
		conn = tls.Client(conn, c.tlsConfig)
	}
	err = c.Connect(conn, "", address)
	if err != nil {
		_ = conn.Close()
		const format = "dial timeout: %s proxy %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, c.scheme, c.address, address)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func (c *Client) Connect(conn net.Conn, _, address string) error {
	_ = conn.SetDeadline(time.Now().Add(c.timeout))
	// CONNECT github.com:443 HTTP/1.1
	// User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:70.0)
	// Connection: keep-alive
	// Host: github.com:443
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintf(buf, "CONNECT %s HTTP/1.1\r\n", address)
	// header
	header := c.header.Clone()
	header.Set("Proxy-Connection", "keep-alive")
	header.Set("Connection", "keep-alive")
	if c.basicAuth != "" {
		header.Set("Proxy-Authorization", c.basicAuth)
	}
	// write header
	for k, v := range header {
		_, _ = fmt.Fprintf(buf, "%s: %s\r\n", k, v[0])
	}
	// host
	_, _ = fmt.Fprintf(buf, "Host: %s\r\n", address)
	// end
	buf.WriteString("\r\n")
	// write to connection
	rAddr := conn.RemoteAddr().String()
	_, err := io.Copy(conn, buf)
	if err != nil {
		return errors.Errorf("failed to write request to %s because %s", rAddr, err)
	}
	// read response
	resp := make([]byte, connectionEstablishedLen)
	_, err = io.ReadAtLeast(conn, resp, connectionEstablishedLen)
	if err != nil {
		return errors.Errorf("failed to read response to %s because %s", rAddr, err)
	}
	// check response
	// HTTP/1.0 200 Connection established
	const format = "%s proxy %s failed to connect to %s"
	p := strings.Split(strings.ReplaceAll(string(resp), "\r\n", ""), " ")
	if len(p) != 4 {
		return errors.Errorf(format, c.scheme, rAddr, address)
	}
	// accept HTTP/1.0 200 Connection established
	//        HTTP/1.1 200 Connection established
	// skip   HTTP/1.0 and HTTP/1.1
	if p[1] == "200" && p[2] == "Connection" && p[3] == "established" {
		return nil
	}
	return errors.Errorf(format, c.scheme, rAddr, address)
}

func (c *Client) HTTP(t *http.Transport) {
	t.Proxy = c.proxy
	// add root CA about https proxy
	if t.TLSClientConfig == nil {
		t.TLSClientConfig = new(tls.Config)
	}
	if t.TLSClientConfig.RootCAs == nil {
		t.TLSClientConfig.RootCAs, _ = cert.SystemCertPool()
	}
	// add certificate for connect https proxy
	for i := 0; i < c.rootCAsLen; i++ {
		t.TLSClientConfig.RootCAs.AddCert(c.rootCAs[i])
	}
}

func (c *Client) Timeout() time.Duration {
	return c.timeout
}

func (c *Client) Address() (string, string) {
	return c.network, c.address
}

// Info is used to get the proxy info
// http://admin:123456@127.0.0.1:8080
// https://admin:123456@[::1]:8081
func (c *Client) Info() string {
	return c.info
}
