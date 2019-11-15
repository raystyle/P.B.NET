package dns

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
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
	defaultMethod = MethodUDP
)

type Server struct {
	Method   string `toml:"method"`
	Address  string `toml:"address"`
	SkipTest bool   `toml:"skip_test"`
}

// Options is used to resolve domain name
type Options struct {
	Mode string `toml:"mode"`

	// if ServerTag != "" ignore it
	Method string `toml:"method"`

	// ipv4 or ipv6
	Type string `toml:"type"`

	// see resolve.go
	Timeout time.Duration `toml:"timeout"`

	// ProxyTag is used to set proxy
	ProxyTag string `toml:"proxy_tag"`

	// ServerTag used to select DNS server
	ServerTag string `toml:"server_tag"`

	// network is useless for DoH
	Network string `toml:"network"`

	// about DoT
	TLSConfig options.TLSConfig `toml:"tls_config"`

	// about DoH, set http.Request Header
	Header http.Header `toml:"header"`

	// about DoH, set http.Client Transport
	Transport options.HTTPTransport `toml:"transport"`

	// MaxBodySize set the max response body that will read
	// about DoH max message size
	MaxBodySize int64 `toml:"max_body_size"`

	// SkipProxy set Options.ProxyTag = ""
	// it still test, but not set proxy
	SkipProxy bool `toml:"skip_proxy"`

	// SkipTest skip all Options test
	SkipTest bool `toml:"skip_test"`

	// about set proxy
	dialContext func(ctx context.Context, network, address string) (net.Conn, error)
	transport   *http.Transport // about DoH
}

// Clone is used to clone dns.Options
func (opts *Options) Clone() *Options {
	optsC := *opts
	optsC.Header = opts.Header.Clone()
	return &optsC
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

func checkNetwork() (enableIPv4, enableIPv6 bool) {
	ifaces, _ := net.Interfaces()
	for i := 0; i < len(ifaces); i++ {
		if ifaces[i].Flags == net.FlagUp|net.FlagBroadcast|net.FlagMulticast {
			addrs, _ := ifaces[i].Addrs()
			for j := 0; j < len(addrs); j++ {
				// check IPv4 or IPv6
				ipAddr := strings.Split(addrs[j].String(), "/")[0]
				ip := net.ParseIP(ipAddr)
				if ip != nil {
					ip4 := ip.To4()
					if ip4 != nil {
						if ip4.IsGlobalUnicast() {
							enableIPv4 = true
						}
					} else {
						if ip.To16().IsGlobalUnicast() {
							enableIPv6 = true
						}
					}
				}
			}
			if enableIPv4 && enableIPv6 {
				break
			}
		}
	}
	return
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

	switch mode {
	case ModeCustom:
		switch opts.Type {
		case "":
			enableIPv4, enableIPv6 := checkNetwork()
			// if network is double stack, prefer IPv6
			if enableIPv4 && enableIPv6 {
				o := opts.Clone()
				o.Type = TypeIPv6
				ipv6, err := c.ResolveWithContext(ctx, domain, o)
				if err != nil && errors.Cause(err) != ErrNoResolveResult { // check options errors
					return nil, err
				}

				o = opts.Clone()
				o.Type = TypeIPv4
				ipv4, _ := c.ResolveWithContext(ctx, domain, o) // don't need check again

				result := append(ipv6, ipv4...)
				if len(result) != 0 {
					return result, nil
				}
				return nil, errors.WithStack(ErrNoResolveResult)
			}
			// IPv4 only
			if enableIPv4 {
				o := opts.Clone()
				o.Type = TypeIPv4
				return c.ResolveWithContext(ctx, domain, o)
			}
			// IPv6 only
			if enableIPv6 {
				o := opts.Clone()
				o.Type = TypeIPv6
				return c.ResolveWithContext(ctx, domain, o)
			}
			return nil, errors.New("network unavailable")
		case TypeIPv4, TypeIPv6:
			return c.customResolve(ctx, domain, opts)
		default:
			return nil, UnknownTypeError(opts.Type)
		}
	case ModeSystem:
		return c.systemResolve(ctx, domain, opts)
	default:
		return nil, errors.Errorf("unknown mode: %s", opts.Mode)
	}
}

func (c *Client) customResolve(
	ctx context.Context,
	domain string,
	opts *Options,
) ([]string, error) {

	cache := c.queryCache(domain, opts.Type)
	if len(cache) != 0 {
		return cache, nil
	}

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
			// apply DoH options (http.Transport)
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

	// resolve
	var result []string
	if opts.ServerTag != "" { // use selected DNS server
		if server, ok := c.Servers()[opts.ServerTag]; ok {
			err = setProxy(server.Method)
			if err != nil {
				return nil, err
			}
			result, err = resolve(ctx, server.Method, server.Address, domain, opts.Type, opts)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errors.Errorf("dns server: %s doesn't exist", opts.ServerTag)
		}
	} else { // query domain name from random DNS server
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
				result, err = resolve(ctx, method, server.Address, domain, opts.Type, opts)
				if err == nil {
					break
				}
			}
		}
	}

	// update cache
	if len(result) != 0 {
		c.updateCache(domain, opts.Type, result)
		return result, nil
	}
	return nil, errors.WithStack(ErrNoResolveResult)
}

func (c *Client) systemResolve(
	ctx context.Context,
	domain string,
	opts *Options,
) ([]string, error) {
	timeout := opts.Timeout
	if timeout < 1 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, err := net.DefaultResolver.LookupHost(ctx, domain)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// check type
	switch opts.Type {
	case "":
		return result, nil
	case TypeIPv4:
		var r []string
		for i := 0; i < len(result); i++ {
			ip := net.ParseIP(result[i]).To4()
			if ip != nil {
				r = append(r, ip.String())
			}
		}
		return r, nil
	case TypeIPv6:
		var r []string
		for i := 0; i < len(result); i++ {
			ip := net.ParseIP(result[i])
			if ip != nil && ip.To4() == nil {
				r = append(r, ip.String())
			}
		}
		return r, nil
	default:
		return nil, UnknownTypeError(opts.Type)
	}
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
