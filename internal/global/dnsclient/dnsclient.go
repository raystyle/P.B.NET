package dnsclient

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"project/internal/dns"
	"project/internal/global/proxyclient"
	"project/internal/options"
)

type Mode string

const (
	CUSTUM Mode = "custom"
	SYSTEM Mode = "system" // usually for intranet dns
)

const (
	default_deadline = 3 * time.Minute
)

var (
	ERROR_INVALID_MODE = errors.New("invalid mode")
)

type Client struct {
	Method  dns.Method `toml:"method"`
	Address string     `toml:"address"`
}

type Options struct {
	Mode  Mode   `toml:"mode"`  // default is custom
	Tag   string `toml:"tag"`   // if tag != "" use selected dns client
	Proxy string `toml:"proxy"` // proxy tag
	// look dns.Options
	Type      dns.Type               `toml:"type"`
	Method    dns.Method             `toml:"method"` // if tag != "" ignore
	Network   string                 `toml:"network"`
	Timeout   time.Duration          `toml:"timeout"`
	Header    http.Header            `toml:"header"`
	Transport options.HTTP_Transport `toml:"transport"`
}

func (this *Options) apply() (*dns.Options, error) {
	opt := &dns.Options{
		Type:    this.Type,
		Method:  this.Method,
		Network: this.Network,
		Timeout: this.Timeout,
		Header:  options.Copy_HTTP_Header(this.Header),
	}
	tr, err := this.Transport.Apply()
	if err != nil {
		return nil, err
	}
	opt.Transport = tr
	return opt, nil
}

type DNS struct {
	proxy       *proxyclient.PROXY // ctx
	deadline    time.Duration
	clients     map[string]*Client // key = tag
	clients_rwm sync.RWMutex
	caches      map[string]*cache // key = domain name
	caches_rwm  sync.RWMutex
}

func New(p *proxyclient.PROXY, c map[string]*Client, deadline time.Duration) (*DNS, error) {
	d := &DNS{
		clients: make(map[string]*Client),
		caches:  make(map[string]*cache),
		proxy:   p,
	}
	// add clients
	for tag, client := range c {
		err := d.Add(tag, client)
		if err != nil {
			return nil, fmt.Errorf("add dns client %s failed: %s", tag, err)
		}
	}
	// set deadline
	if deadline <= 0 {
		deadline = default_deadline
	}
	err := d.Set_Cache_Deadline(deadline)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// TODO test

// first select custom or system to resolve dns
// second set domain & options
func (this *DNS) Resolve(domain string, opts *Options) ([]string, error) {
	if opts == nil {
		opts = new(Options)
	}
	_type := opts.Type
	if _type == "" {
		_type = dns.IPV4
	}
	cache_list := this.query_cache(domain, _type)
	if cache_list != nil {
		return cache_list, nil
	}
	var ip_list []string
	switch opts.Mode {
	case "", CUSTUM:
		dns_opts, err := opts.apply()
		if err != nil {
			return nil, err
		}
		// set proxy
		proxy, err := this.proxy.Get(opts.Proxy)
		if err != nil {
			return nil, err
		}
		if proxy != nil {
			switch opts.Method {
			case "", dns.TLS, dns.UDP, dns.TCP:
				dns_opts.Dial = proxy.Dial
			case dns.DOH:
				proxy.HTTP(dns_opts.Transport)
			default:
				return nil, dns.ERR_UNKNOWN_METHOD
			}
		}
		// check tag
		if opts.Tag != "" {
			this.clients_rwm.RLock()
			client, ok := this.clients[opts.Tag]
			this.clients_rwm.RUnlock()
			if !ok {
				return nil, fmt.Errorf("dns client: %s doesn't exist", opts.Tag)
			}
			dns_opts.Method = client.Method
			return dns.Resolve(client.Address, domain, dns_opts)
		}
		// query dns
		m := opts.Method
		if m == "" {
			m = dns.DEFAULT_METHOD
		}
		// copy map
		clients := make(map[string]*Client)
		this.clients_rwm.RLock()
		for tag, client := range this.clients {
			clients[tag] = client
		}
		this.clients_rwm.RUnlock()
		for _, client := range clients {
			if client.Method == m {
				ip_list, err = dns.Resolve(client.Address, domain, dns_opts)
				if err == nil {
					break
				}
			}
		}
	case SYSTEM:
		var err error
		ip_list, err = system_resolve(domain, _type)
		if err != nil {
			return nil, err
		}
	default:
		return nil, ERROR_INVALID_MODE
	}
	if ip_list != nil {
		switch _type {
		case dns.IPV4:
			this.update_cache(domain, ip_list, nil)
		case dns.IPV6:
			this.update_cache(domain, nil, ip_list)
		}
		return ip_list, nil
	}
	return nil, dns.ERR_NO_RESOLVE_RESULT
}

func (this *DNS) Clients() map[string]*Client {
	client_pool := make(map[string]*Client)
	this.clients_rwm.RLock()
	for tag, client := range this.clients {
		client_pool[tag] = client
	}
	this.clients_rwm.RUnlock()
	return client_pool
}

func (this *DNS) Add(tag string, c *Client) error {
	switch c.Method {
	case dns.TLS, dns.UDP, dns.TCP, dns.DOH:
	default:
		return dns.ERR_UNKNOWN_METHOD
	}
	defer this.clients_rwm.Unlock()
	this.clients_rwm.Lock()
	if _, exist := this.clients[tag]; !exist {
		this.clients[tag] = c
		return nil
	} else {
		return fmt.Errorf("dns client: %s already exists", tag)
	}
}

func (this *DNS) Delete(tag string) error {
	defer this.clients_rwm.Unlock()
	this.clients_rwm.Lock()
	if _, exist := this.clients[tag]; exist {
		delete(this.clients, tag)
		return nil
	} else {
		return fmt.Errorf("dns client: %s doesn't exist", tag)
	}
}
