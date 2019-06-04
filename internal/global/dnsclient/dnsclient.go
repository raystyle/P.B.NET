package dnsclient

import (
	"errors"
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
	Method  dns.Method
	Address string
}

type Options struct {
	Mode  Mode
	Proxy string
	Opts  dns.Options
	// for dns.Options.Transport
	H_Transport *options.HTTP_Transport
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
			return nil, err
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

// first select custom or system to resolve dns
// second set domain & options
func (this *DNS) Resolve(domain string, opts *Options) ([]string, error) {
	if opts == nil {
		opts = new(Options)
	}
	cache_list := this.query_cache(domain, opts.Opts.Type)
	if cache_list != nil {
		return cache_list, nil
	}
	var (
		ip_list []string
		err     error
	)
	switch opts.Mode {
	case "", CUSTUM:
		// copy map
		clients := make(map[string]*Client)
		this.clients_rwm.RLock()
		for tag, client := range this.clients {
			clients[tag] = client
		}
		this.clients_rwm.RUnlock()
		// set dns options
		if opts.H_Transport != nil {
			opts.Opts.Transport, err = opts.H_Transport.Apply()
			if err != nil {
				return nil, err
			}
		}
		// set proxy
		p, err := this.proxy.Get(opts.Proxy)
		if err != nil {
			return nil, err
		}
		if p != nil {
			switch opts.Opts.Method {
			case dns.TLS, dns.UDP, dns.TCP:
				opts.Opts.Dial = p.Dial
			case dns.DOH:
				if opts.Opts.Transport == nil {
					opts.Opts.Transport, _ = new(options.HTTP_Transport).Apply()
				}
				p.HTTP(opts.Opts.Transport)
			default:
				return nil, dns.ERR_UNKNOWN_METHOD
			}
		}
		// query dns
		for _, client := range clients {
			if client.Method == opts.Opts.Method {
				ip_list, err = dns.Resolve(client.Address, domain, &opts.Opts)
				if err == nil {
					break
				}
			}
		}
	case SYSTEM:
		ip_list, err = system_resolve(domain, opts.Opts.Type)
	default:
		return nil, ERROR_INVALID_MODE
	}
	if err == nil {
		switch opts.Opts.Type {
		case 0, dns.IPV4:
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
	case "", dns.TLS, dns.UDP, dns.TCP, dns.DOH:
	default:
		return dns.ERR_UNKNOWN_METHOD
	}
	defer this.clients_rwm.Unlock()
	this.clients_rwm.Lock()
	if _, exist := this.clients[tag]; !exist {
		this.clients[tag] = c
		return nil
	} else {
		return errors.New("dns client: " + tag + " already exists")
	}
}

func (this *DNS) Delete(tag string) error {
	defer this.clients_rwm.Unlock()
	this.clients_rwm.Lock()
	if _, exist := this.clients[tag]; exist {
		delete(this.clients, tag)
		return nil
	} else {
		return errors.New("dns client: " + tag + " doesn't exist")
	}
}

func (this *DNS) Destroy() {
	this.clients_rwm.Lock()
	this.clients = make(map[string]*Client)
	this.clients_rwm.Unlock()
	this.caches_rwm.Lock()
	this.caches = make(map[string]*cache)
	this.caches_rwm.Unlock()
}
