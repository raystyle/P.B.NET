package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"

	"project/internal/options"
)

type Client struct {
	network   string
	address   string
	https     bool
	header    http.Header
	tlsConfig *tls.Config
	timeout   time.Duration

	proxyURL  *url.URL
	proxy     func(*http.Request) (*url.URL, error)
	basicAuth string
}

func NewClient(network, address string, https bool, opts *Options) (*Client, error) {
	client := Client{
		network: network,
		address: address,
		https:   https,
		header:  opts.Header.Clone(),
		timeout: opts.Timeout,
	}

	if client.header == nil {
		client.header = make(http.Header)
	}

	if https {
		var err error
		client.tlsConfig, err = opts.TLSConfig.Apply()
		if err != nil {
			return nil, errors.WithStack(err)
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
	u := &url.URL{
		Scheme: "http",
		Host:   address,
	}
	if https {
		u.Scheme = "https"
	}
	if opts.Username != "" && opts.Password != "" {
		u.User = url.UserPassword(opts.Username, opts.Password)
		auth := u.User.String()
		client.basicAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	} else if opts.Username != "" {
		u.User = url.User(opts.Username)
		auth := u.User.String()
		client.basicAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	}
	// check proxy url
	var err error
	u, err = url.Parse(u.String())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	client.proxy = http.ProxyURL(u)
	client.proxyURL = u
	return &client, nil
}

func (c *Client) Dial(_, address string) (net.Conn, error) {
	conn, err := (&net.Dialer{Timeout: c.timeout}).Dial(c.network, c.address)
	if err != nil {
		const format = "dial: connect http proxy %s failed"
		return nil, errors.Wrapf(err, format, c.address)
	}
	if c.https {
		conn = tls.Client(conn, c.tlsConfig)
	}
	err = c.Connect(conn, "", address)
	if err != nil {
		_ = conn.Close()
		const format = "dial: http proxy %s connect %s failed"
		return nil, errors.WithMessagef(err, format, c.address, address)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func (c *Client) DialContext(ctx context.Context, _, address string) (net.Conn, error) {
	conn, err := (&net.Dialer{Timeout: c.timeout}).DialContext(ctx, c.network, c.address)
	if err != nil {
		const format = "dial context: connect http proxy %s failed"
		return nil, errors.Wrapf(err, format, c.address)
	}
	if c.https {
		conn = tls.Client(conn, c.tlsConfig)
	}
	err = c.Connect(conn, "", address)
	if err != nil {
		_ = conn.Close()
		const format = "dial context: http proxy %s connect %s failed"
		return nil, errors.WithMessagef(err, format, c.address, address)
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
		const format = "dial timeout: connect http proxy %s failed"
		return nil, errors.Wrapf(err, format, c.address)
	}
	if c.https {
		conn = tls.Client(conn, c.tlsConfig)
	}
	err = c.Connect(conn, "", address)
	if err != nil {
		_ = conn.Close()
		const format = "dial timeout: http proxy %s connect %s failed"
		return nil, errors.WithMessagef(err, format, c.address, address)
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
	p := strings.Split(strings.ReplaceAll(string(resp), "\r\n", ""), " ")
	if len(p) != 4 {
		return errors.Errorf("http proxy %s failed to connect to %s", rAddr, address)
	}
	// HTTP/1.0 200 Connection established or HTTP/1.1 200 Connection established
	if p[1] == "200" && p[2] == "Connection" && p[3] == "established" {
		return nil
	}
	return errors.Errorf("http proxy %s failed to connect to %s", rAddr, address)
}

func (c *Client) HTTP(t *http.Transport) {
	t.Proxy = c.proxy
}

func (c *Client) Info() string {
	return c.proxyURL.String()
}
