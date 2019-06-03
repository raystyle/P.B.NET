package httpproxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"
)

var (
	ERR_NOT_SUPPORT_DIAL = errors.New("http proxy not support dial")
)

type Client struct {
	url   *url.URL
	proxy func(*http.Request) (*url.URL, error)
}

// config = url "http://username:password@127.0.0.1:8080"
func New_Client(config string) (*Client, error) {
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

func (this *Client) Dial(_, _ string) (net.Conn, error) {
	return nil, ERR_NOT_SUPPORT_DIAL
}

func (this *Client) Dial_Context(_ context.Context, _, _ string) (net.Conn, error) {
	return nil, ERR_NOT_SUPPORT_DIAL
}

func (this *Client) Dial_Timeout(_, _ string, _ time.Duration) (net.Conn, error) {
	return nil, ERR_NOT_SUPPORT_DIAL
}

func (this *Client) HTTP(t *http.Transport) {
	t.Proxy = this.proxy
}

func (this *Client) Info() string {
	return this.url.String()
}
