package proxyclient

import (
	"errors"
	"fmt"
	"sync"

	"github.com/pelletier/go-toml"

	"project/internal/proxy"
	"project/internal/proxy/httpproxy"
	"project/internal/proxy/socks5"
)

var (
	ERR_RESERVE_PROXY = errors.New("direct is reserve proxy")
	ERR_UNKNOWN_MODE  = errors.New("unknown mode")
)

type Client struct {
	Mode         proxy.Mode
	Config       string // toml or other
	proxy.Client `toml:"-"`
}

type PROXY struct {
	clients map[string]*Client // key = tag
	rwm     sync.RWMutex
}

// key = tag
func New(c map[string]*Client) (*PROXY, error) {
	p := &PROXY{
		clients: make(map[string]*Client),
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
func (this *PROXY) Get(tag string) (*Client, error) {
	if tag == "" || tag == "direct" {
		return nil, nil
	}
	defer this.rwm.RUnlock()
	this.rwm.RLock()
	if p, exist := this.clients[tag]; exist {
		return p, nil
	} else {
		return nil, fmt.Errorf("proxy: %s doesn't exist", tag)
	}
}

func (this *PROXY) Clients() map[string]*Client {
	clients := make(map[string]*Client)
	defer this.rwm.RUnlock()
	this.rwm.RLock()
	for tag, p := range this.clients {
		clients[tag] = p
	}
	return clients
}

func (this *PROXY) Add(tag string, c *Client) error {
	if tag == "" || tag == "direct" {
		return ERR_RESERVE_PROXY
	}
	switch c.Mode {
	case proxy.SOCKS5:
		conf := &struct {
			Clients []*socks5.Config
		}{}
		err := toml.Unmarshal([]byte(c.Config), conf)
		if err != nil {
			return err
		}
		pc, err := socks5.New_Client(conf.Clients...)
		if err != nil {
			return err
		}
		c.Client = pc
	case proxy.HTTP:
		pc, err := httpproxy.New_Client(c.Config)
		if err != nil {
			return err
		}
		c.Client = pc
	default:
		return ERR_UNKNOWN_MODE
	}
	defer this.rwm.Unlock()
	this.rwm.Lock()
	if _, exist := this.clients[tag]; !exist {
		this.clients[tag] = c
		return nil
	} else {
		return fmt.Errorf("proxy: %s already exists", tag)
	}
}

func (this *PROXY) Delete(tag string) error {
	if tag == "" || tag == "direct" {
		return ERR_RESERVE_PROXY
	}
	defer this.rwm.Unlock()
	this.rwm.Lock()
	if _, exist := this.clients[tag]; exist {
		delete(this.clients, tag)
		return nil
	} else {
		return fmt.Errorf("proxy: %s doesn't exist", tag)
	}
}
