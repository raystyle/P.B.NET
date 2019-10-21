package dns

import (
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
	DoT Method = "dot" // DNS-Over-TLS
	DoH Method = "doh" // DNS-Over-HTTPS
)

const (
	defaultMethod = UDP
)

type UnknownMethodError string

func (m UnknownMethodError) Error() string {
	return fmt.Sprintf("unknown method: %s", string(m))
}

type Server struct {
	Method  Method `toml:"method"`
	Address string `toml:"address"`
}

type Client struct {
	proxyPool *proxy.Pool

	expire     time.Duration      // cache expire time
	servers    map[string]*Server // key = tag
	serversRWM sync.RWMutex
	caches     map[string]*cache // key = domain name
	cachesRWM  sync.RWMutex
}

func NewClient(
	pool *proxy.Pool,
	servers map[string]*Server,
	expire time.Duration,
) (*Client, error) {
	client := Client{
		proxyPool: pool,
		servers:   make(map[string]*Server),
		caches:    make(map[string]*cache),
	}
	// add clients
	for tag, server := range servers {
		err := client.Add(tag, server)
		if err != nil {
			return nil, fmt.Errorf("add dns server %s failed: %s", tag, err)
		}
	}
	err := client.SetCacheExpireTime(expire)
	if err != nil {
		return nil, err
	}
	return &client, nil
}

func (c *Client) Add(tag string, server *Server) error {
	switch server.Method {
	case UDP, TCP, DoT, DoH:
	default:
		return UnknownMethodError(server.Method)
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

func (c *Client) Servers() map[string]*Server {
	servers := make(map[string]*Server)
	c.serversRWM.RLock()
	for tag, server := range c.servers {
		servers[tag] = server
	}
	c.serversRWM.RUnlock()
	return servers
}

// TODO test
func (c *Client) Test() error {
	return nil
}

type Options struct {
	Mode        Mode                  `toml:"mode"`          // default is custom
	Method      Method                `toml:"method"`        // if ServerTag != "" ignore it
	Type        Type                  `toml:"type"`          // default is IPv4
	Timeout     time.Duration         `toml:"timeout"`       // dial and xnet.DeadlineConn
	ServerTag   string                `toml:"server_tag"`    // if != "" use selected dns client
	ProxyTag    string                `toml:"proxy_tag"`     // proxy tag
	Network     string                `toml:"network"`       // useless for DOH
	Header      http.Header           `toml:"header"`        // about DOH
	Transport   options.HTTPTransport `toml:"transport"`     // about DOH
	MaxBodySize int64                 `toml:"max_body_size"` // about DOH max message size

	dial      func(network, address string, timeout time.Duration) (net.Conn, error)
	transport *http.Transport // about DOH
}

// select custom or system to resolve dns
// set domain & options
func (c *Client) Resolve(domain string, opts *Options) ([]string, error) {
	if opts == nil {
		opts = new(Options)
	}
	Type := opts.Type
	if Type == "" {
		Type = IPv4
	}
	// first query caches
	cache := c.queryCache(domain, Type)
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
		if opts.Method == DoH {
			opts.transport, err = opts.Transport.Apply()
			if err != nil {
				return nil, err
			}
		}
		// set proxy
		p, err := c.proxyPool.Get(opts.ProxyTag)
		if err != nil {
			return nil, err
		}
		switch opts.Method {
		case "", UDP, TCP, DoT:
			opts.dial = p.DialTimeout
		case DoH:
			p.HTTP(opts.transport)
		default:
			return nil, UnknownMethodError(opts.Method)
		}
		// check tag exist
		if opts.ServerTag != "" {
			c.serversRWM.RLock()
			if server, ok := c.servers[opts.ServerTag]; !ok {
				c.serversRWM.RUnlock()
				return nil, fmt.Errorf("dns server: %s doesn't exist", opts.ServerTag)
			} else {
				c.serversRWM.RUnlock()
				opts.Method = server.Method
				return customResolve(server.Address, domain, opts)
			}
		}
		// query dns
		method := opts.Method
		if method == "" {
			method = defaultMethod
		}
		for _, server := range c.Servers() {
			if server.Method == method {
				result, err = customResolve(server.Address, domain, opts)
				if err == nil {
					break
				}
			}
		}
	case System:
		result, err = systemResolve(domain, Type)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown mode: %s", opts.Mode)
	}
	// update cache
	if len(result) != 0 {
		switch Type {
		case IPv4:
			c.updateCache(domain, result, nil)
		case IPv6:
			c.updateCache(domain, nil, result)
		}
		return result, nil
	}
	return nil, ErrNoResolveResult
}
