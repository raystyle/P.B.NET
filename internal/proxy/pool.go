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
	Direct Mode = "direct"
	Socks5 Mode = "socks5"
	HTTP   Mode = "http"
)

var (
	ErrEmptyTag      = errors.New("proxy client tag is empty")
	ErrReserveTag    = errors.New("direct is reserve tag")
	ErrReserveClient = errors.New("direct is reserve proxy client")
)

type Client struct {
	Mode   Mode
	Config string
	client
}

type client interface {
	Dial(network, address string) (net.Conn, error)
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
	HTTP(transport *http.Transport)
	Info() string
}

type Pool struct {
	clients map[string]*Client // key = tag
	rwm     sync.RWMutex
}

// NewPool is used to create a proxy pool for role.global
func NewPool(clients map[string]*Client) (*Pool, error) {
	pool := Pool{clients: make(map[string]*Client)}
	// add proxy clients
	for tag, client := range clients {
		err := pool.Add(tag, client)
		if err != nil {
			return nil, fmt.Errorf("add proxy client %s failed: %s", tag, err)
		}
	}
	// add direct
	dc := &Client{
		Mode:   Direct,
		client: new(direct.Direct),
	}
	pool.clients[""] = dc
	pool.clients["direct"] = dc
	return &pool, nil
}

// Add is used to add a proxy client
func (p *Pool) Add(tag string, client *Client) error {
	if tag == "" {
		return ErrEmptyTag
	}
	if tag == "direct" {
		return ErrReserveTag
	}
	switch client.Mode {
	case Direct:
		return nil
	case Socks5:
		conf := &struct {
			Clients []*socks5.Config `toml:"server"`
		}{}
		err := toml.Unmarshal([]byte(client.Config), conf)
		if err != nil {
			return err
		}
		c, err := socks5.NewClient(conf.Clients...)
		if err != nil {
			return err
		}
		client.client = c
	case HTTP:
		c, err := hp.NewClient(client.Config)
		if err != nil {
			return err
		}
		client.client = c
	default:
		return fmt.Errorf("unknown mode: %s", client.Mode)
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
func (p *Pool) Get(tag string) (*Client, error) {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	if client, ok := p.clients[tag]; ok {
		return client, nil
	} else {
		return nil, fmt.Errorf("proxy client: %s doesn't exist", tag)
	}
}

// Clients is used to get all proxy clients
func (p *Pool) Clients() map[string]*Client {
	clients := make(map[string]*Client)
	p.rwm.RLock()
	for tag, client := range p.clients {
		clients[tag] = client
	}
	p.rwm.RUnlock()
	return clients
}
