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
	timeout   time.Duration
	header    http.Header
	tlsConfig *tls.Config

	rootCAs    []*x509.Certificate
	rootCAsLen int

	scheme    string // "http" or "https"
	proxy     func(*http.Request) (*url.URL, error)
	basicAuth string
	info      string
}

// NewHTTPClient is used to create a HTTP proxy client
func NewHTTPClient(network, address string, opts *Options) (*Client, error) {
	return newClient(network, address, opts, false)
}

// NewHTTPSClient is used to create a HTTPS proxy client
func NewHTTPSClient(network, address string, opts *Options) (*Client, error) {
	return newClient(network, address, opts, true)
}

func newClient(network, address string, opts *Options, https bool) (*Client, error) {
	err := CheckNetwork(network)
	if err != nil {
		return nil, err
	}

	if opts == nil {
		opts = new(Options)
	}

	client := Client{
		network: network,
		address: address,
		https:   https,
		timeout: opts.Timeout,
		header:  opts.Header.Clone(),
	}

	if client.https {
		var err error
		client.tlsConfig, err = opts.TLSConfig.Apply()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		// copy certificates
		client.rootCAs, _ = opts.TLSConfig.GetRootCAs()
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

	if client.header == nil {
		client.header = make(http.Header)
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

	client.proxy = http.ProxyURL(u)
	client.info = u.String()
	return &client, nil
}

// Dial is used to connect to address through proxy
func (c *Client) Dial(network, address string) (net.Conn, error) {
	err := CheckNetwork(network)
	if err != nil {
		return nil, err
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
	err := CheckNetwork(network)
	if err != nil {
		return nil, err
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
	err := CheckNetwork(network)
	if err != nil {
		return nil, err
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
	err := CheckNetwork(network)
	if err != nil {
		return nil, err
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
	defer func() {
		close(done)
		wg.Wait()
	}()

	// write to connection
	rAddr := conn.RemoteAddr().String()
	_, err = io.Copy(conn, buf)
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
	// add certificates if connect https proxy server
	if c.https {
		if t.TLSClientConfig == nil {
			t.TLSClientConfig = new(tls.Config)
		}
		if t.TLSClientConfig.RootCAs == nil {
			t.TLSClientConfig.RootCAs = x509.NewCertPool()
		}
		for i := 0; i < c.rootCAsLen; i++ {
			t.TLSClientConfig.RootCAs.AddCert(c.rootCAs[i])
		}
		// add client certificates, if certificate exists, will not add again
		for _, cert := range c.tlsConfig.Certificates {
			contain := false
			cc := cert.Certificate[0]
			for _, tCert := range t.TLSClientConfig.Certificates {
				if bytes.Compare(cc, tCert.Certificate[0]) == 0 {
					contain = true
					break
				}
			}
			if !contain {
				certs := t.TLSClientConfig.Certificates
				t.TLSClientConfig.Certificates = append(certs, cert)
			}
		}
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
//
// http://admin:123456@127.0.0.1:8080
// https://admin:123456@[::1]:8081
func (c *Client) Info() string {
	return c.info
}
