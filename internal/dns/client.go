package dns

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/options"
	"project/internal/proxy"
)

type Mode string

const (
	Custom Mode = "custom"
	System Mode = "system"
)

type UnknownTypeError string

func (t UnknownTypeError) Error() string {
	return fmt.Sprintf("unknown type: %s", string(t))
}

// resolve method
type Method = string

const (
	UDP Method = "udp"
	TCP Method = "tcp"
	DoT Method = "dot" // DNS-Over-TLS
	DoH Method = "doh" // DNS-Over-HTTPS
)

type UnknownMethodError string

func (m UnknownMethodError) Error() string {
	return fmt.Sprintf("unknown method: %s", string(m))
}

const (
	defaultMode   = Custom
	defaultType   = IPv4
	defaultMethod = UDP
)

type Server struct {
	Method   Method `toml:"method"`
	Address  string `toml:"address"`
	SkipTest bool   `toml:"skip_test"`
}

// Options is used to Resolve domain name
type Options struct {
	Mode Mode `toml:"mode"`

	// if ServerTag != "" ignore it
	Method Method `toml:"method"`

	// ipv4 or ipv6
	Type string `toml:"type"`

	Timeout time.Duration `toml:"timeout"`

	// ServerTag used to select DNS server
	ServerTag string `toml:"server_tag"`

	// ProxyTag is used to set proxy
	ProxyTag string `toml:"proxy_tag"`

	// network is useless for DOH
	Network string `toml:"network"`

	// about DOH, set http.Request Header
	Header http.Header `toml:"header"`

	// about DOH, set http.Client Transport
	Transport options.HTTPTransport `toml:"transport"`

	// MaxBodySize set the max response body that will read
	// about DOH max message size
	MaxBodySize int64 `toml:"max_body_size"`

	// SkipProxy set Options.ProxyTag = ""
	// it still test, but not set proxy
	SkipProxy bool `toml:"skip_proxy"`

	// SkipTest skip all Options test
	SkipTest bool `toml:"skip_test"`

	// context
	dial      func(network, address string, timeout time.Duration) (net.Conn, error)
	transport *http.Transport // about DOH
}

type Client struct {
	proxyPool *proxy.Pool

	expire     time.Duration      // cache expire time, default is 5 minute
	servers    map[string]*Server // key = tag
	serversRWM sync.RWMutex
	caches     map[string]*cache // key = domain name
	cachesRWM  sync.RWMutex
}

func NewClient(pool *proxy.Pool) *Client {
	return &Client{
		proxyPool: pool,
		expire:    options.DefaultCacheExpireTime,
		servers:   make(map[string]*Server),
		caches:    make(map[string]*cache),
	}
}

func (c *Client) Add(tag string, server *Server) error {
	err := c.add(tag, server)
	if err != nil {
		return errors.WithMessagef(err, "failed to add dns server %s", tag)
	}
	return nil
}

func (c *Client) add(tag string, server *Server) error {
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
		return errors.New("already exists")
	}
}

func (c *Client) Delete(tag string) error {
	c.serversRWM.Lock()
	defer c.serversRWM.Unlock()
	if _, ok := c.servers[tag]; ok {
		delete(c.servers, tag)
		return nil
	} else {
		return errors.Errorf("dns server: %s doesn't exist", tag)
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

// Resolve is used to resolve domain name with options
// select custom or system to resolve dns
// set domain & options
func (c *Client) Resolve(domain string, opts *Options) ([]string, error) {
	if opts == nil {
		opts = new(Options)
	}

	mode := opts.Mode
	if mode == "" {
		mode = defaultMode
	}

	typ := opts.Type
	if typ == "" {
		typ = defaultType
	}

	// check type
	switch typ {
	case IPv4, IPv6:
	default:
		return nil, UnknownTypeError(typ)
	}

	// first query from caches
	cache := c.queryCache(domain, typ)
	l := len(cache)
	if l != 0 {
		// must copy
		cp := make([]string, l)
		copy(cp, cache)
		return cp, nil
	}

	// apply options and resolve domain name
	var (
		result []string
		err    error
	)
	switch mode {
	case Custom:
		method := opts.Method
		if method == "" {
			method = defaultMethod
		}

		// apply doh options (http.Transport)
		if method == DoH {
			opts.transport, err = opts.Transport.Apply()
			if err != nil {
				return nil, err
			}
		}

		// set proxy and check method
		p, err := c.proxyPool.Get(opts.ProxyTag)
		if err != nil {
			return nil, err
		}
		switch method {
		case UDP, TCP, DoT:
			opts.dial = p.DialTimeout
		case DoH:
			p.HTTP(opts.transport)
		default:
			return nil, UnknownMethodError(method)
		}

		// check tag exist
		if opts.ServerTag != "" {
			c.serversRWM.RLock()
			if server, ok := c.servers[opts.ServerTag]; ok {
				c.serversRWM.RUnlock()
				method = server.Method
				return customResolve(method, server.Address, domain, typ, opts)
			} else {
				c.serversRWM.RUnlock()
				return nil, errors.Errorf("dns server: %s doesn't exist", opts.ServerTag)
			}
		}

		// query dns from random dns server
		for _, server := range c.Servers() {
			if server.Method == method {
				result, err = customResolve(method, server.Address, domain, typ, opts)
				if err == nil {
					break
				}
			}
		}
	case System:
		result, err = systemResolve(typ, domain)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.Errorf("unknown mode: %s", opts.Mode)
	}

	// update cache
	l = len(result)
	if l != 0 {
		// must copy
		cp := make([]string, l)
		copy(cp, result)
		switch typ {
		case IPv4:
			c.updateCache(domain, cp, nil)
		case IPv6:
			c.updateCache(domain, nil, cp)
		}
		return result, nil
	}
	return nil, ErrNoResolveResult
}

func (c *Client) TestDNSServers(domain, typ string) error {
	for tag, server := range c.Servers() {
		if server.SkipTest {
			continue
		}
		opts := &Options{
			Type:    typ,
			Timeout: 10 * time.Second,
		}
		// set server tag to use DNS server that selected
		opts.ServerTag = tag
		_, err := c.Resolve(domain, opts)
		if err != nil {
			return errors.WithMessagef(err, "failed to test dns server: %s", tag)
		}
	}
	return nil
}

func (c *Client) TestOptions(domain string, opts *Options) error {
	if opts.SkipTest {
		return nil
	}
	if opts.SkipProxy {
		opts.ProxyTag = ""
	}
	_, err := c.Resolve(domain, opts)
	if err != nil {
		return errors.WithMessage(err, "failed to test option")
	}
	return nil
}
