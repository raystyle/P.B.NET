package dns

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"project/internal/options"
	"project/internal/proxy"
	"project/internal/xpanic"
)

// supported modes
const (
	ModeCustom = "custom"
	ModeSystem = "system"
)

// UnknownTypeError is an error of the type
type UnknownTypeError string

func (t UnknownTypeError) Error() string {
	return fmt.Sprintf("unknown type: %s", string(t))
}

// supported custom resolve methods
const (
	MethodUDP = "udp"
	MethodTCP = "tcp"
	MethodDoT = "dot" // DNS-Over-TLS
	MethodDoH = "doh" // DNS-Over-HTTPS
)

// UnknownMethodError is an error of the method
type UnknownMethodError string

func (m UnknownMethodError) Error() string {
	return fmt.Sprintf("unknown method: %s", string(m))
}

const (
	defaultMode   = ModeCustom
	defaultMethod = MethodUDP

	defaultCacheExpireTime = time.Minute
)

// errors
var (
	ErrInvalidExpireTime = fmt.Errorf("expire time < 10 seconds or > 10 minutes")
	ErrNoDNSServers      = fmt.Errorf("no DNS servers")
)

// Server include DNS server info
type Server struct {
	Method   string `toml:"method"`
	Address  string `toml:"address"`
	SkipTest bool   `toml:"skip_test"`
}

// Options contains resolve options
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

// Client is a DNS client that support various DNS server
type Client struct {
	proxyPool *proxy.Pool

	expire time.Duration // cache expire time, default is 5 minute

	servers    map[string]*Server // key = tag
	serversRWM sync.RWMutex

	enableCache atomic.Value      // usually for TestServers
	caches      map[string]*cache // key = domain name
	cachesRWM   sync.RWMutex
}

// NewClient is used to create a DNS client
func NewClient(pool *proxy.Pool) *Client {
	c := Client{
		proxyPool: pool,
		expire:    defaultCacheExpireTime,
		servers:   make(map[string]*Server),
		caches:    make(map[string]*cache),
	}
	c.EnableCache()
	return &c
}

// Add is used to add a DNS server
func (c *Client) Add(tag string, server *Server) error {
	const format = "failed to add DNS server %s"
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
	}
	return errors.New("already exists")
}

// Delete is used to delete a DNS server
func (c *Client) Delete(tag string) error {
	c.serversRWM.Lock()
	defer c.serversRWM.Unlock()
	if _, ok := c.servers[tag]; ok {
		delete(c.servers, tag)
		return nil
	}
	return errors.Errorf("DNS server %s doesn't exist", tag)
}

// Servers is used to get all DNS Servers
func (c *Client) Servers() map[string]*Server {
	c.serversRWM.RLock()
	defer c.serversRWM.RUnlock()
	servers := make(map[string]*Server, len(c.servers))
	for tag, server := range c.servers {
		servers[tag] = server
	}
	return servers
}

func checkNetwork() (enableIPv4, enableIPv6 bool) {
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Flags != net.FlagUp|net.FlagBroadcast|net.FlagMulticast {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ipAddr := strings.Split(addr.String(), "/")[0]
			ip := net.ParseIP(ipAddr)
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
func (c *Client) ResolveWithContext(ctx context.Context, domain string, opts *Options) ([]string, error) {
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
			return c.selectType(ctx, domain, opts)
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

func (c *Client) selectType(ctx context.Context, domain string, opts *Options) ([]string, error) {
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
}

func (c *Client) setProxy(p *proxy.Client, method string, opts *Options) error {
	switch method {
	case MethodUDP, MethodTCP, MethodDoT:
		opts.dialContext = p.DialContext
	case MethodDoH:
		// apply DoH options (http.Transport)
		var err error
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

func (c *Client) customResolve(
	ctx context.Context,
	domain string,
	opts *Options,
) ([]string, error) {

	if c.isEnableCache() {
		cache := c.queryCache(domain, opts.Type)
		if len(cache) != 0 {
			return cache, nil
		}
	}

	// set proxy and check method
	p, err := c.proxyPool.Get(opts.ProxyTag)
	if err != nil {
		return nil, err
	}

	// resolve
	var result []string
	if opts.ServerTag != "" { // use selected DNS server
		if server, ok := c.Servers()[opts.ServerTag]; ok {
			err = c.setProxy(p, server.Method, opts)
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
		err = c.setProxy(p, method, opts)
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
	if len(result) == 0 {
		return nil, errors.WithStack(ErrNoResolveResult)
	}
	// update cache
	if c.isEnableCache() {
		c.updateCache(domain, opts.Type, result)
	}
	return result, nil
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

func (c *Client) isEnableCache() bool {
	return c.enableCache.Load().(bool)
}

// EnableCache is used to enable cache
func (c *Client) EnableCache() {
	c.enableCache.Store(true)
}

// DisableCache is used to disable cache
func (c *Client) DisableCache() {
	c.enableCache.Store(false)
}

// GetCacheExpireTime is used to get cache expire time
func (c *Client) GetCacheExpireTime() time.Duration {
	c.cachesRWM.RLock()
	defer c.cachesRWM.RUnlock()
	expire := c.expire
	return expire
}

// SetCacheExpireTime is used to set cache expire time
func (c *Client) SetCacheExpireTime(expire time.Duration) error {
	if expire < 10*time.Second || expire > 10*time.Minute {
		return ErrInvalidExpireTime
	}
	c.cachesRWM.Lock()
	defer c.cachesRWM.Unlock()
	c.expire = expire
	return nil
}

// FlushCache is used to flush all cache
func (c *Client) FlushCache() {
	c.cachesRWM.Lock()
	defer c.cachesRWM.Unlock()
	c.caches = make(map[string]*cache)
}

// TestServers is used to test all DNS servers
func (c *Client) TestServers(ctx context.Context, domain string, opts *Options) ([]string, error) {
	l := len(c.servers)
	if l == 0 {
		return nil, ErrNoDNSServers
	}
	c.DisableCache()
	defer c.EnableCache()
	results := make(map[string]struct{}) // remove duplicate result
	resultsM := sync.Mutex{}
	errChan := make(chan error, l)
	for tag, server := range c.servers {
		if server.SkipTest {
			errChan <- nil
			continue
		}
		go func(tag string) {
			var err error
			defer func() {
				if r := recover(); r != nil {
					err = xpanic.Error(r, "Client.TestServers")
				}
				errChan <- err
			}()
			// set server tag to use DNS server that selected
			o := opts.Clone()
			o.ServerTag = tag
			result, err := c.ResolveWithContext(ctx, domain, o)
			if err != nil {
				err = errors.WithMessagef(err, "failed to test server %s", tag)
				return
			}
			resultsM.Lock()
			defer resultsM.Unlock()
			for i := 0; i < len(result); i++ {
				results[result[i]] = struct{}{}
			}
		}(tag)
	}
	// get errors
	for i := 0; i < l; i++ {
		select {
		case err := <-errChan:
			if err != nil {
				return nil, err
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	close(errChan)
	result := make([]string, 0, len(results)/l+2)
	for ip := range results {
		result = append(result, ip)
	}
	return result, nil
}

// TestOption is used to test Options
func (c *Client) TestOption(ctx context.Context, domain string, opts *Options) ([]string, error) {
	if opts.SkipTest {
		return nil, nil
	}
	c.DisableCache()
	defer c.EnableCache()
	o := opts.Clone()
	if o.SkipProxy {
		o.ProxyTag = ""
	}
	result, err := c.ResolveWithContext(ctx, domain, o)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to test option")
	}
	return result, nil
}
