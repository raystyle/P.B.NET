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
	"sync"
	"time"

	"github.com/pkg/errors"
)

const defaultDialTimeout = 30 * time.Second

// Client implement internal/proxy.client
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

// NewClient is used to create HTTP proxy client
func NewClient(network, address string, opts *Options) (*Client, error) {
	// check network
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, errors.Errorf("unsupported network: %s", network)
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
		// copy certs
		client.rootCAs, _ = opts.TLSConfig.RootCA()
		client.rootCAsLen = len(client.rootCAs)
		// set server name
		if client.tlsConfig.ServerName == "" {
			colonPos := strings.LastIndex(address, ":")
			if colonPos == -1 {
				return nil, errors.New("missing port in address")
			}
			hostname := address[:colonPos]
			c := client.tlsConfig.Clone()
			c.ServerName = hostname
			client.tlsConfig = c
		}
	}

	if client.timeout < 1 {
		client.timeout = defaultDialTimeout
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
	if opts.Username != "" || opts.Password != "" {
		u.User = url.UserPassword(opts.Username, opts.Password)
		auth := []byte(u.User.String())
		client.basicAuth = "Basic " + base64.StdEncoding.EncodeToString(auth)
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

// Dial is used to connect to address through proxy
func (c *Client) Dial(network, address string) (net.Conn, error) {
	// check network
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, errors.Errorf("unsupported network: %s", network)
	}
	conn, err := (&net.Dialer{Timeout: c.timeout}).Dial(c.network, c.address)
	if err != nil {
		const format = "dial: failed to connect %s proxy %s"
		return nil, errors.Wrapf(err, format, c.scheme, c.address)
	}
	pConn, err := c.Connect(context.Background(), conn, network, address)
	if err != nil {
		_ = conn.Close()
		const format = "dial: %s proxy %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, c.scheme, c.address, address)
	}
	_ = pConn.SetDeadline(time.Time{})
	return pConn, nil
}

// DialContext is used to connect to address through proxy with context
func (c *Client) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	// check network
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, errors.Errorf("unsupported network: %s", network)
	}
	conn, err := (&net.Dialer{Timeout: c.timeout}).DialContext(ctx, c.network, c.address)
	if err != nil {
		const format = "dial context: failed to connect %s proxy %s"
		return nil, errors.Wrapf(err, format, c.scheme, c.address)
	}
	pConn, err := c.Connect(ctx, conn, network, address)
	if err != nil {
		_ = conn.Close()
		const format = "dial context: %s proxy %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, c.scheme, c.address, address)
	}
	_ = pConn.SetDeadline(time.Time{})
	return pConn, nil
}

// DialTimeout is used to connect to address through proxy with timeout
func (c *Client) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	// check network
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, errors.Errorf("unsupported network: %s", network)
	}
	if timeout < 1 {
		timeout = defaultDialTimeout
	}
	conn, err := (&net.Dialer{Timeout: timeout}).Dial(c.network, c.address)
	if err != nil {
		const format = "dial timeout: failed to connect %s proxy %s"
		return nil, errors.Wrapf(err, format, c.scheme, c.address)
	}
	pConn, err := c.Connect(context.Background(), conn, network, address)
	if err != nil {
		_ = conn.Close()
		const format = "dial timeout: %s proxy %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, c.scheme, c.address, address)
	}
	_ = pConn.SetDeadline(time.Time{})
	return pConn, nil
}

// Connect is used to connect to address through proxy with context
func (c *Client) Connect(
	ctx context.Context,
	conn net.Conn,
	network string,
	address string,
) (net.Conn, error) {
	// check network
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, errors.Errorf("unsupported network: %s", network)
	}
	if c.https {
		conn = tls.Client(conn, c.tlsConfig)
	}
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

	// interrupt
	wg := sync.WaitGroup{}
	done := make(chan struct{})
	defer func() {
		close(done)
		wg.Wait()
	}()
	wg.Add(1)
	go func() {
		defer func() {
			recover()
			wg.Done()
		}()
		select {
		case <-done:
		case <-ctx.Done():
			_ = conn.Close()
		}
	}()

	// write to connection
	rAddr := conn.RemoteAddr().String()
	_, err := io.Copy(conn, buf)
	if err != nil {
		return nil, errors.Errorf("failed to write request to %s because %s", rAddr, err)
	}
	// read response
	resp := make([]byte, connectionEstablishedLen)
	_, err = io.ReadAtLeast(conn, resp, connectionEstablishedLen)
	if err != nil {
		return nil, errors.Errorf("failed to read response from %s because %s", rAddr, err)
	}
	// check response
	// HTTP/1.0 200 Connection established
	const format = "%s proxy %s failed to connect to %s"
	p := strings.Split(strings.ReplaceAll(string(resp), "\r\n", ""), " ")
	if len(p) != 4 {
		return nil, errors.Errorf(format, c.scheme, c.address, address)
	}
	// accept HTTP/1.0 200 Connection established
	//        HTTP/1.1 200 Connection established
	// skip   HTTP/1.0 and HTTP/1.1
	if p[1] == "200" && p[2] == "Connection" && p[3] == "established" {
		return conn, nil
	}
	return nil, errors.Errorf(format, c.scheme, c.address, address)
}

// HTTP is used to set *http.Transport about proxy
func (c *Client) HTTP(t *http.Transport) {
	t.Proxy = c.proxy
	// add root CA about https proxy
	if t.TLSClientConfig == nil {
		t.TLSClientConfig = new(tls.Config)
	}
	if t.TLSClientConfig.RootCAs == nil {
		t.TLSClientConfig.RootCAs = x509.NewCertPool()
	}
	// add certificate for connect https proxy
	for i := 0; i < c.rootCAsLen; i++ {
		t.TLSClientConfig.RootCAs.AddCert(c.rootCAs[i])
	}
}

// Timeout is used to get the proxy client timeout
func (c *Client) Timeout() time.Duration {
	return c.timeout
}

// Server is used to get the proxy server address
func (c *Client) Server() (string, string) {
	return c.network, c.address
}

// Info is used to get the proxy client info
// http://admin:123456@127.0.0.1:8080
// https://admin:123456@[::1]:8081
func (c *Client) Info() string {
	return c.info
}
