package dns

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/options"
	"project/internal/proxy"
)

const (
	ModeCustom = "custom"
	ModeSystem = "system"
)

type UnknownTypeError string

func (t UnknownTypeError) Error() string {
	return fmt.Sprintf("unknown type: %s", string(t))
}

const (
	MethodUDP = "udp"
	MethodTCP = "tcp"
	MethodDoT = "dot" // DNS-Over-TLS
	MethodDoH = "doh" // DNS-Over-HTTPS
)

type UnknownMethodError string

func (m UnknownMethodError) Error() string {
	return fmt.Sprintf("unknown method: %s", string(m))
}

const (
	defaultMode   = ModeCustom
	defaultType   = TypeIPv4
	defaultMethod = MethodUDP
)

type Server struct {
	Method   string `toml:"method"`
	Address  string `toml:"address"`
	SkipTest bool   `toml:"skip_test"`
}

// Options is used to Resolve domain name
type Options struct {
	Mode string `toml:"mode"`

	// if ServerTag != "" ignore it
	Method string `toml:"method"`

	// ipv4 or ipv6
	Type string `toml:"type"`

	Timeout time.Duration `toml:"timeout"`

	// ProxyTag is used to set proxy
	ProxyTag string `toml:"proxy_tag"`

	// ServerTag used to select DNS server
	ServerTag string `toml:"server_tag"`

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
	dialContext func(ctx context.Context, network, address string) (net.Conn, error)
	transport   *http.Transport // about DOH
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
	const format = "failed to add dns server %s"
	return errors.WithMessagef(c.add(tag, server), format, tag)
}

func (c *Client) add(tag string, server *Server) error {
	switch server.Method {
	case MethodUDP, MethodTCP, MethodDoT, MethodDoH:
	default:
		return errors.WithStack(UnknownMethodError(server.Method))
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
	defer c.serversRWM.RUnlock()
	for tag, server := range c.servers {
		servers[tag] = server
	}
	return servers
}

// Resolve is used to resolve domain name
// select custom or system to resolve dns
// set domain & options
func (c *Client) Resolve(domain string, opts *Options) ([]string, error) {
	return c.ResolveWithContext(context.Background(), domain, opts)
}

// ResolveWithContext is used to resolve domain name with context
func (c *Client) ResolveWithContext(
	ctx context.Context,
	domain string,
	opts *Options,
) ([]string, error) {
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
	case TypeIPv4, TypeIPv6:
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
	case ModeCustom:
		// set proxy and check method
		p, err := c.proxyPool.Get(opts.ProxyTag)
		if err != nil {
			return nil, err
		}
		setProxy := func(method string) error {
			switch method {
			case MethodUDP, MethodTCP, MethodDoT:
				opts.dialContext = p.DialContext
			case MethodDoH:
				// apply doh options (http.Transport)
				opts.transport, err = opts.Transport.Apply()
				if err != nil {
					return err
				}
				p.HTTP(opts.transport)
			default:
				return UnknownMethodError(method)
			}
			return nil
		}

		// check server tag
		if opts.ServerTag != "" {
			if server, ok := c.Servers()[opts.ServerTag]; ok {
				err = setProxy(server.Method)
				if err != nil {
					return nil, err
				}
				return customResolve(ctx, server.Method, server.Address, domain, typ, opts)
			} else {
				return nil, errors.Errorf("dns server: %s doesn't exist", opts.ServerTag)
			}
		}

		// query dns from random dns server
		method := opts.Method
		if method == "" {
			method = defaultMethod
		}
		err = setProxy(method)
		if err != nil {
			return nil, err
		}
		for _, server := range c.Servers() {
			if server.Method == method {
				result, err = customResolve(ctx, method, server.Address, domain, typ, opts)
				if err == nil {
					break
				}
			}
		}
	case ModeSystem:
		timeout := opts.Timeout
		if timeout < 1 {
			timeout = 3 * time.Second
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		result, err = systemResolve(ctx, typ, domain)
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
		case TypeIPv4:
			c.updateCache(domain, cp, nil)
		case TypeIPv6:
			c.updateCache(domain, nil, cp)
		}
		return result, nil
	}
	return nil, errors.WithStack(ErrNoResolveResult)
}

func (c *Client) TestServers(ctx context.Context, domain string, opts *Options) ([]string, error) {
	var result []string
	for tag, server := range c.Servers() {
		c.FlushCache()
		if server.SkipTest {
			continue
		}
		// set server tag to use DNS server that selected
		opts.ServerTag = tag
		var err error
		result, err = c.ResolveWithContext(ctx, domain, opts)
		if err != nil {
			return nil, errors.WithMessagef(err, "failed to test dns server %s", tag)
		}
	}
	return result, nil
}

func (c *Client) TestOptions(ctx context.Context, domain string, opts *Options) ([]string, error) {
	if opts.SkipTest {
		return nil, nil
	}
	c.FlushCache()
	if opts.SkipProxy {
		opts.ProxyTag = ""
	}
	result, err := c.ResolveWithContext(ctx, domain, opts)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to test dns option")
	}
	return result, nil
}
