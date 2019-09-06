package dns

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"project/internal/options"
	"project/internal/proxy"
)

type Mode string

const (
	Custom Mode = "custom"
	System Mode = "system"
)

// resolve method
type Method = string

const (
	UDP Method = "udp"
	TCP Method = "tcp"
	TLS Method = "tls" // DNS-Over-TLS
	DOH Method = "doh" // DNS-Over-HTTPS
)

var (
	ErrUnknownMode = errors.New("unknown mode")
)

type Server struct {
	Method  Method `toml:"method"`
	Address string `toml:"address"`
}

type Client struct {
	pool       *proxy.Pool
	deadline   time.Duration      // cache expire time
	servers    map[string]*Server // key = tag
	serversRWM sync.RWMutex
	caches     map[string]*cache // key = domain name
	cachesRWM  sync.RWMutex
}

func NewClient(p *proxy.Pool, s map[string]*Server, deadline time.Duration) (*Client, error) {
	client := Client{
		pool:    p,
		servers: make(map[string]*Server),
		caches:  make(map[string]*cache),
	}
	// add clients
	for tag, server := range s {
		err := client.Add(tag, server)
		if err != nil {
			return nil, fmt.Errorf("add dns server %s failed: %s", tag, err)
		}
	}
	// set deadline
	if deadline < 1 {
		deadline = defaultCacheDeadline
	}
	err := client.SetCacheDeadline(deadline)
	if err != nil {
		return nil, err
	}
	return &client, nil
}

// TODO test
func (c *Client) Test() error {
	return nil
}

type Options struct {
	Mode      Mode                  `toml:"mode"`   // default is custom
	Method    Method                `toml:"method"` // default is UDP if != "" ignore it
	Type      Type                  `toml:"type"`   // default is IPv4
	Timeout   time.Duration         `toml:"timeout"`
	ServerTag string                `toml:"server_tag"` // if != "" use selected dns client
	ProxyTag  string                `toml:"proxy_tag"`  // proxy tag
	Network   string                `toml:"network"`
	Header    http.Header           `toml:"header"`    // about DOH
	Transport options.HTTPTransport `toml:"transport"` // about DOH

	dial      func(network, address string) (net.Conn, error) // for proxy, useless for doh
	transport *http.Transport                                 // about DOH
}

// select custom or system to resolve dns
// set domain & options
func (c *Client) Resolve(domain string, opts *Options) ([]string, error) {
	if opts == nil {
		opts = new(Options)
	}
	_type := opts.Type
	if _type == "" {
		_type = IPv4
	}
	// first query caches
	cache := c.queryCache(domain, _type)
	if cache != nil {
		return cache, nil
	}
	var (
		result []string
		err    error
	)
	switch opts.Mode {
	case "", Custom:
		// apply doh options(http Transport)
		if opts.Method == DOH {
			opts.transport, err = opts.Transport.Apply()
			if err != nil {
				return nil, err
			}
		}
		// set proxy
		p, err := c.pool.Get(opts.ProxyTag)
		if err != nil {
			return nil, err
		}
		if p != nil {
			switch opts.Method {
			case "", UDP, TCP, TLS:
				opts.dial = p.Dial
			case DOH:
				p.HTTP(opts.transport)
			default:
				return nil, ErrUnknownMethod
			}
		}
		// check tag exist
		if opts.ServerTag != "" {
			c.serversRWM.RLock()
			defer c.serversRWM.RUnlock()
			if server, ok := c.servers[opts.ServerTag]; !ok {
				return nil, fmt.Errorf("dns server: %s doesn't exist", opts.ServerTag)
			} else {
				opts.Method = server.Method
				return resolve(server.Address, domain, opts)
			}
		}
		// query dns
		_method := opts.Method
		if _method == "" {
			_method = defaultMethod
		}
		for _, server := range c.Servers() {
			if server.Method == _method {
				result, err = resolve(server.Address, domain, opts)
				if err == nil {
					break
				}
			}
		}
	case System:
		result, err = systemResolve(domain, _type)
		if err != nil {
			return nil, err
		}
	default:
		return nil, ErrUnknownMode
	}
	// update cache
	if result != nil {
		switch _type {
		case IPv4:
			c.updateCache(domain, result, nil)
		case IPv6:
			c.updateCache(domain, nil, result)
		}
		return result, nil
	}
	return nil, ErrNoResolveResult
}

func (c *Client) Servers() map[string]*Server {
	servers := make(map[string]*Server)
	c.serversRWM.RLock()
	for tag, server := range c.servers {
		servers[tag] = server
	}
	c.serversRWM.RUnlock()
	return servers
}

func (c *Client) Add(tag string, server *Server) error {
	switch server.Method {
	case UDP, TCP, TLS, DOH:
	default:
		return ErrUnknownMethod
	}
	c.serversRWM.Lock()
	defer c.serversRWM.Unlock()
	if _, ok := c.servers[tag]; !ok {
		c.servers[tag] = server
		return nil
	} else {
		return fmt.Errorf("dns server: %s already exists", tag)
	}
}

func (c *Client) Delete(tag string) error {
	c.serversRWM.Lock()
	defer c.serversRWM.Unlock()
	if _, ok := c.servers[tag]; ok {
		delete(c.servers, tag)
		return nil
	} else {
		return fmt.Errorf("dns server: %s doesn't exist", tag)
	}
}
