package http

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"
)

var (
	ErrNotSupportDial = errors.New("http proxy not support dial")
)

type Client struct {
	url   *url.URL
	proxy func(*http.Request) (*url.URL, error)
}

// config = url "http://username:password@127.0.0.1:8080"
func NewClient(config string) (*Client, error) {
	u, err := url.Parse(config)
	if err != nil {
		return nil, err
	}
	return &Client{
		url: u,
		proxy: func(*http.Request) (*url.URL, error) {
			return u, nil
		},
	}, nil
}

func (c *Client) Dial(_, _ string) (net.Conn, error) {
	return nil, ErrNotSupportDial
}

func (c *Client) DialContext(_ context.Context, _, _ string) (net.Conn, error) {
	return nil, ErrNotSupportDial
}

func (c *Client) DialTimeout(_, _ string, _ time.Duration) (net.Conn, error) {
	return nil, ErrNotSupportDial
}

func (c *Client) HTTP(t *http.Transport) {
	t.Proxy = c.proxy
}

func (c *Client) Info() string {
	return c.url.String()
}
