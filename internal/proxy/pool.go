package proxy

import (
	"sync"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	"project/internal/proxy/direct"
	"project/internal/proxy/http"
	"project/internal/proxy/socks"
)

type Pool struct {
	clients map[string]*Client // key = tag
	rwm     sync.RWMutex
}

// NewPool is used to create a proxy client pool
func NewPool(clients map[string]*Client) (*Pool, error) {
	pool := Pool{clients: make(map[string]*Client)}
	// add proxy clients
	for tag, client := range clients {
		err := pool.Add(tag, client)
		if err != nil {
			return nil, errors.WithMessagef(err, "failed to add %s:", tag)
		}
	}
	// add direct
	dc := &Client{
		Mode:   ModeDirect,
		client: new(direct.Direct),
	}
	pool.clients[""] = dc
	pool.clients["direct"] = dc
	return &pool, nil
}

// Add is used to add a proxy client
func (p *Pool) Add(tag string, client *Client) error {
	if tag == "" {
		return errors.New("empty proxy client tag")
	}
	if tag == "direct" {
		return errors.New("direct is the reserve proxy client tag")
	}
	switch client.Mode {
	case ModeSocks:
		opts := new(socks.Options)
		err := toml.Unmarshal([]byte(client.Options), opts)
		if err != nil {
			return errors.WithStack(err)
		}
		c, err := socks.NewClient(client.Network, client.Address, opts)
		if err != nil {
			return err
		}
		client.client = c
	case ModeHTTP:
		opts := new(http.Options)
		err := toml.Unmarshal([]byte(client.Options), opts)
		if err != nil {
			return errors.WithStack(err)
		}
		c, err := http.NewClient(client.Network, client.Address, opts)
		if err != nil {
			return err
		}
		client.client = c
	default:
		return errors.Errorf("unknown mode %s", client.Mode)
	}
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if _, ok := p.clients[tag]; !ok {
		p.clients[tag] = client
		return nil
	} else {
		return errors.Errorf("proxy client %s already exists", tag)
	}
}

// Delete is used to delete proxy client
func (p *Pool) Delete(tag string) error {
	if tag == "" {
		return errors.New("empty proxy client tag")
	}
	if tag == "direct" {
		return errors.New("direct is the reserve proxy client")
	}
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if _, ok := p.clients[tag]; ok {
		delete(p.clients, tag)
		return nil
	} else {
		return errors.Errorf("proxy client %s doesn't exist", tag)
	}
}

// Get is used to get proxy client
// return Direct if tag is "" or "direct"
func (p *Pool) Get(tag string) (*Client, error) {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	if client, ok := p.clients[tag]; ok {
		return client, nil
	} else {
		return nil, errors.Errorf("proxy client %s doesn't exist", tag)
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
