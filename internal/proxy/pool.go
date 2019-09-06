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

	"project/internal/proxy/httpproxy"
	"project/internal/proxy/socks5"
)

type Mode = string

const (
	Socks5 Mode = "socks5"
	HTTP   Mode = "http"
)

var (
	ErrReserveProxy = errors.New("direct is reserve proxy")
	ErrUnknownMode  = errors.New("unknown mode")
)

type client interface {
	Dial(network, address string) (net.Conn, error)
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
	HTTP(transport *http.Transport)
	Info() string
}

type Client struct {
	Mode   Mode   `toml:"mode"`
	Config string `toml:"config"` // toml or other, see proxy/*
	client
}

type Pool struct {
	clients map[string]*Client // key = tag
	rwm     sync.RWMutex
}

// NewPool is for x.global
func NewPool(c map[string]*Client) (*Pool, error) {
	p := Pool{
		clients: make(map[string]*Client),
	}
	for tag, client := range c {
		err := p.Add(tag, client)
		if err != nil {
			return nil, fmt.Errorf("add proxy client %s failed: %s", tag, err)
		}
	}
	return &p, nil
}

// if tag = "" return Direct
func (p *Pool) Get(tag string) (*Client, error) {
	if tag == "" || tag == "direct" {
		return nil, nil
	}
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	if client, ok := p.clients[tag]; ok {
		return client, nil
	} else {
		return nil, fmt.Errorf("proxy client: %s doesn't exist", tag)
	}
}

func (p *Pool) Clients() map[string]*Client {
	clients := make(map[string]*Client)
	p.rwm.RLock()
	for tag, client := range p.clients {
		clients[tag] = client
	}
	p.rwm.RUnlock()
	return clients
}

func (p *Pool) Add(tag string, client *Client) error {
	if tag == "" || tag == "direct" {
		return ErrReserveProxy
	}
	switch client.Mode {
	case Socks5:
		conf := &struct {
			Clients []*socks5.Config
		}{}
		err := toml.Unmarshal([]byte(client.Config), conf)
		if err != nil {
			return err
		}
		pc, err := socks5.NewClient(conf.Clients...)
		if err != nil {
			return err
		}
		client.client = pc
	case HTTP:
		pc, err := httpproxy.NewClient(client.Config)
		if err != nil {
			return err
		}
		client.client = pc
	default:
		return ErrUnknownMode
	}
	// <security> set client config to ""
	client.Config = ""
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if _, ok := p.clients[tag]; !ok {
		p.clients[tag] = client
		return nil
	} else {
		return fmt.Errorf("proxy client: %s already exists", tag)
	}
}

func (p *Pool) Delete(tag string) error {
	if tag == "" || tag == "direct" {
		return ErrReserveProxy
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
