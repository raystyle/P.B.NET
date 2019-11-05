package proxy

import (
	"sync"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	"project/internal/proxy/direct"
	"project/internal/proxy/http"
	"project/internal/proxy/socks"
)

// Pool is a proxy client pool
type Pool struct {
	clients map[string]*Client // key = tag
	rwm     sync.RWMutex
}

// NewPool is used to create a proxy client pool
func NewPool() *Pool {
	pool := Pool{clients: make(map[string]*Client)}
	// add direct
	dc := &Client{
		Tag:    ModeDirect,
		Mode:   ModeDirect,
		client: new(direct.Direct),
	}
	pool.clients[""] = dc
	pool.clients["direct"] = dc
	return &pool
}

// Add is used to add a proxy client
func (p *Pool) Add(client *Client) error {
	err := p.add(client)
	if err != nil {
		return errors.WithMessagef(err, "failed to add proxy client %s:", client.Tag)
	}
	return nil
}

func (p *Pool) add(client *Client) error {
	if client.Tag == "" {
		return errors.New("empty proxy client tag")
	}
	if client.Tag == ModeDirect {
		return errors.New("direct is the reserve proxy client tag")
	}
	switch client.Mode {
	case ModeSocks:
		opts := new(socks.Options)
		if client.Options != "" {
			err := toml.Unmarshal([]byte(client.Options), opts)
			if err != nil {
				return errors.WithStack(err)
			}
		}
		c, err := socks.NewClient(client.Network, client.Address, opts)
		if err != nil {
			return err
		}
		client.client = c
	case ModeHTTP:
		opts := new(http.Options)
		if client.Options != "" {
			err := toml.Unmarshal([]byte(client.Options), opts)
			if err != nil {
				return errors.WithStack(err)
			}
		}
		c, err := http.NewClient(client.Network, client.Address, opts)
		if err != nil {
			return err
		}
		client.client = c
	case ModeChain:
		tags := struct {
			Tags []string `toml:"tags"`
		}{} // client tags
		err := toml.Unmarshal([]byte(client.Options), &tags)
		if err != nil {
			return errors.WithStack(err)
		}
		var clients []*Client
		for i := 0; i < len(tags.Tags); i++ {
			client, err := p.Get(tags.Tags[i])
			if err != nil {
				return err
			}
			clients = append(clients, client)
		}
		c, err := NewChain(client.Tag, clients...)
		if err != nil {
			return err
		}
		client.client = c
	case ModeBalance:
		tags := struct {
			Tags []string `toml:"tags"`
		}{} // client tags
		err := toml.Unmarshal([]byte(client.Options), &tags)
		if err != nil {
			return errors.WithStack(err)
		}
		var clients []*Client
		for i := 0; i < len(tags.Tags); i++ {
			client, err := p.Get(tags.Tags[i])
			if err != nil {
				return err
			}
			clients = append(clients, client)
		}
		c, err := NewBalance(client.Tag, clients...)
		if err != nil {
			return err
		}
		client.client = c
	default:
		return errors.Errorf("unknown mode %s", client.Mode)
	}
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if _, ok := p.clients[client.Tag]; !ok {
		p.clients[client.Tag] = client
		return nil
	} else {
		return errors.Errorf("proxy client %s already exists", client.Tag)
	}
}

// Delete is used to delete proxy client
func (p *Pool) Delete(tag string) error {
	if tag == "" {
		return errors.New("empty proxy client tag")
	}
	if tag == ModeDirect {
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
	defer p.rwm.RUnlock()
	for tag, client := range p.clients {
		clients[tag] = client
	}
	return clients
}
