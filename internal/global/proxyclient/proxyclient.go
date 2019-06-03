package proxyclient

import (
	"errors"
	"fmt"
	"sync"

	"project/internal/proxy"
)

var (
	ERR_RESERVE_PROXY = errors.New("direct is reserve proxy")
)

type Client struct {
	Mode   proxy.Mode
	Config interface{}
}

type PROXY struct {
	clients map[string]proxy.Client // key = tag
	rwmutex sync.RWMutex
}

// key = tag
func New(c map[string]*Client) (*PROXY, error) {
	p := &PROXY{
		clients: make(map[string]proxy.Client),
	}
	for tag, client := range c {
		err := p.Add(tag, client)
		if err != nil {
			return nil, fmt.Errorf("add proxy %s failed: %s", tag, err)
		}
	}
	return p, nil
}

// if tag = "" return Direct
func (this *PROXY) Get(tag string) (proxy.Client, error) {
	if tag == "" || tag == "direct" {
		return nil, nil
	}
	defer this.rwmutex.RUnlock()
	this.rwmutex.RLock()
	if p, exist := this.clients[tag]; exist {
		return p, nil
	} else {
		return nil, fmt.Errorf("proxy: %s doesn't exist", tag)
	}
}

func (this *PROXY) Clients() map[string]proxy.Client {
	clients := make(map[string]proxy.Client)
	defer this.rwmutex.RUnlock()
	this.rwmutex.RLock()
	for tag, p := range this.clients {
		clients[tag] = p
	}
	return clients
}

func (this *PROXY) Add(tag string, c *Client) error {
	if tag == "" || tag == "direct" {
		return ERR_RESERVE_PROXY
	}
	client, err := proxy.Load_Client(c.Mode, c.Config)
	if err != nil {
		return err
	}
	defer this.rwmutex.Unlock()
	this.rwmutex.Lock()
	if _, exist := this.clients[tag]; !exist {
		this.clients[tag] = client
		return nil
	} else {
		return fmt.Errorf("proxy: %s already exists", tag)
	}
}

func (this *PROXY) Delete(tag string) error {
	if tag == "" || tag == "direct" {
		return ERR_RESERVE_PROXY
	}
	defer this.rwmutex.Unlock()
	this.rwmutex.Lock()
	if _, exist := this.clients[tag]; exist {
		delete(this.clients, tag)
		return nil
	} else {
		return fmt.Errorf("proxy: %s doesn't exist", tag)
	}
}

func (this *PROXY) Destroy() {
	this.rwmutex.Lock()
	this.clients = make(map[string]proxy.Client)
	this.rwmutex.Unlock()
}
