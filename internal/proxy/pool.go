package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/pelletier/go-toml"

	"project/internal/proxy/direct"
	hp "project/internal/proxy/http"
	"project/internal/proxy/socks5"
)

type Mode = string

const (
	Socks5 Mode = "socks5"
	HTTP   Mode = "http"
)

var (
	ErrEmptyTag      = errors.New("proxy client tag is empty")
	ErrReserveTag    = errors.New("direct is reserve tag")
	ErrReserveClient = errors.New("direct is reserve proxy client")
)

type Client interface {
	Dial(network, address string) (net.Conn, error)
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
	HTTP(transport *http.Transport)
	Info() string
	Mode() string
}

type Pool struct {
	clients map[string]Client // key = tag
	rwm     sync.RWMutex
}

// NewPool is used to create a proxy pool for role.global
func NewPool() *Pool {
	p := Pool{clients: make(map[string]Client)}
	// set direct
	d := new(direct.Direct)
	p.clients[""] = d
	p.clients["direct"] = d
	return &p
}

// Add is used to add a proxy client
func (p *Pool) Add(tag string, mode Mode, config string) error {
	if tag == "" {
		return ErrEmptyTag
	}
	if tag == "direct" {
		return ErrReserveTag
	}
	var client Client
	switch mode {
	case Socks5:
		conf := &struct {
			Clients []*socks5.Config
		}{}
		err := toml.Unmarshal([]byte(config), conf)
		if err != nil {
			return err
		}
		c, err := socks5.NewClient(conf.Clients...)
		if err != nil {
			return err
		}
		client = c
	case HTTP:
		c, err := hp.NewClient(config)
		if err != nil {
			return err
		}
		client = c
	default:
		return fmt.Errorf("unknown mode: %s", mode)
	}
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if _, ok := p.clients[tag]; !ok {
		p.clients[tag] = client
		return nil
	} else {
		return fmt.Errorf("proxy client: %s already exists", tag)
	}
}

// Delete is used to delete proxy client
func (p *Pool) Delete(tag string) error {
	if tag == "" {
		return ErrEmptyTag
	}
	if tag == "direct" {
		return ErrReserveClient
	}
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if _, ok := p.clients[tag]; ok {
		delete(p.clients, tag)
		return nil
	} else {
		return fmt.Errorf("proxy client: %s doesn't exist", tag)
	}
}

// Get is used to get proxy client, if tag is "" or "direct" return Direct
func (p *Pool) Get(tag string) (Client, error) {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	if client, ok := p.clients[tag]; ok {
		return client, nil
	} else {
		return nil, fmt.Errorf("proxy client: %s doesn't exist", tag)
	}
}

// Clients is used to get all proxy clients
func (p *Pool) Clients() map[string]Client {
	clients := make(map[string]Client)
	p.rwm.RLock()
	for tag, client := range p.clients {
		clients[tag] = client
	}
	p.rwm.RUnlock()
	return clients
}
