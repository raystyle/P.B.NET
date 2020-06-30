package proxy

import (
	"sync"

	"github.com/pkg/errors"

	"project/internal/cert"
	"project/internal/patch/toml"
	"project/internal/proxy/direct"
	"project/internal/proxy/http"
	"project/internal/proxy/socks"
)

// Pool is the proxy client pool.
type Pool struct {
	certPool *cert.Pool

	// key = tag
	clients    map[string]*Client
	clientsRWM sync.RWMutex
}

// NewPool is used to create a proxy client pool.
func NewPool(certPool *cert.Pool) *Pool {
	pool := Pool{
		certPool: certPool,
		clients:  make(map[string]*Client),
	}
	// add direct proxy client(reserved)
	dc := &Client{
		Tag:    ModeDirect,
		Mode:   ModeDirect,
		client: new(direct.Direct),
	}
	pool.clients[""] = dc
	pool.clients["direct"] = dc
	return &pool
}

// Add is used to add a proxy client.
func (p *Pool) Add(client *Client) error {
	err := p.add(client)
	if err != nil {
		const format = "failed to add proxy client %s"
		return errors.WithMessagef(err, format, client.Tag)
	}
	return nil
}

func (p *Pool) add(client *Client) error {
	if client.Tag == "" {
		return errors.New("empty tag")
	}
	if client.Tag == ModeDirect {
		return errors.New("direct is the reserve tag")
	}
	if client.Mode == "" {
		return errors.New("empty mode")
	}
	if client.Mode != ModeChain && client.Mode != ModeBalance && client.Address == "" {
		return errors.New("empty address")
	}
	var err error
	switch client.Mode {
	case ModeSocks5, ModeSocks4a, ModeSocks4:
		err = p.addSocks(client)
	case ModeHTTP, ModeHTTPS:
		err = p.addHTTP(client)
	case ModeChain:
		err = p.addChain(client)
	case ModeBalance:
		err = p.addBalance(client)
	default:
		return errors.Errorf("unknown mode: %s", client.Mode)
	}
	if err != nil {
		return err
	}
	p.clientsRWM.Lock()
	defer p.clientsRWM.Unlock()
	if _, ok := p.clients[client.Tag]; !ok {
		p.clients[client.Tag] = client
		return nil
	}
	return errors.New("already exists")
}

func (p *Pool) addSocks(client *Client) error {
	opts := new(socks.Options)
	if client.Options != "" {
		err := toml.Unmarshal([]byte(client.Options), opts)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	var err error
	switch client.Mode {
	case ModeSocks5:
		client.client, err = socks.NewSocks5Client(client.Network, client.Address, opts)
	case ModeSocks4a:
		client.client, err = socks.NewSocks4aClient(client.Network, client.Address, opts)
	case ModeSocks4:
		client.client, err = socks.NewSocks4Client(client.Network, client.Address, opts)
	}
	return err
}

func (p *Pool) addHTTP(client *Client) error {
	opts := new(http.Options)
	if client.Options != "" {
		err := toml.Unmarshal([]byte(client.Options), opts)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	var err error
	switch client.Mode {
	case ModeHTTP:
		client.client, err = http.NewHTTPClient(client.Network, client.Address, opts)
	case ModeHTTPS:
		opts.Server.TLSConfig.CertPool = p.certPool
		opts.Transport.TLSClientConfig.CertPool = p.certPool
		client.client, err = http.NewHTTPSClient(client.Network, client.Address, opts)
	}
	return err
}

func (p *Pool) addChain(client *Client) error {
	tags := struct {
		Tags []string `toml:"tags"`
	}{}
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
	client.client, err = NewChain(client.Tag, clients...)
	return err
}

func (p *Pool) addBalance(client *Client) error {
	tags := struct {
		Tags []string `toml:"tags"`
	}{}
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
	client.client, err = NewBalance(client.Tag, clients...)
	return err
}

// Delete is used to delete proxy client.
func (p *Pool) Delete(tag string) error {
	if tag == "" {
		return errors.New("empty proxy client tag")
	}
	if tag == ModeDirect {
		return errors.New("direct is the reserve proxy client")
	}
	p.clientsRWM.Lock()
	defer p.clientsRWM.Unlock()
	if _, ok := p.clients[tag]; ok {
		delete(p.clients, tag)
		return nil
	}
	return errors.Errorf("proxy client %s doesn't exist", tag)
}

// Get is used to get a proxy client.
// return Direct if tag is "" or "direct"
func (p *Pool) Get(tag string) (*Client, error) {
	p.clientsRWM.RLock()
	defer p.clientsRWM.RUnlock()
	if client, ok := p.clients[tag]; ok {
		return client, nil
	}
	return nil, errors.Errorf("proxy client %s doesn't exist", tag)
}

// Clients is used to get all proxy clients.
func (p *Pool) Clients() map[string]*Client {
	p.clientsRWM.RLock()
	defer p.clientsRWM.RUnlock()
	clients := make(map[string]*Client, len(p.clients))
	for tag, client := range p.clients {
		clients[tag] = client
	}
	return clients
}
