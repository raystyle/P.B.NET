package client

import (
	"sync"

	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/proxy/socks"
)

type Configs struct {
	Service struct {
		Name        string `toml:"name"`
		DisplayName string `toml:"display_name"`
		Description string `toml:"description"`
	} `toml:"service"`

	Listener struct {
		Network  string `toml:"network"`
		Address  string `toml:"address"`
		Username string `toml:"username"`
		Password string `toml:"password"`
		MaxConns int    `toml:"max_conns"`
	} `toml:"listener"`

	Clients []*struct {
		Tag     string `toml:"tag"`
		Mode    string `toml:"mode"`
		Network string `toml:"network"`
		Address string `toml:"address"`
		Options string `toml:"options"`
	} `toml:"clients"`
}

type Client struct {
	tag      string
	configs  *Configs
	server   *socks.Server
	stopOnce sync.Once
}

func New(tag string, config *Configs) *Client {
	return &Client{tag: tag, configs: config}
}

func (client *Client) Start() error {
	pool := proxy.NewPool()
	for _, client := range client.configs.Clients {
		err := pool.Add(&proxy.Client{
			Tag:     client.Tag,
			Mode:    client.Mode,
			Network: client.Network,
			Address: client.Address,
			Options: client.Options,
		})
		if err != nil {
			return err
		}
	}
	// if tag, use the last proxy client
	if client.tag == "" {
		client.tag = client.configs.Clients[len(client.configs.Clients)-1].Tag
	}

	// set proxy client
	pc, err := pool.Get(client.tag)
	if err != nil {
		return err
	}

	// start socks5 server
	lc := client.configs.Listener
	opts := socks.Options{
		Username:    lc.Username,
		Password:    lc.Password,
		MaxConns:    lc.MaxConns,
		DialTimeout: pc.DialTimeout,
	}
	client.server, err = socks.NewServer("proxy", logger.Test, &opts)
	if err != nil {
		return err
	}
	return client.server.ListenAndServe(lc.Network, lc.Address)
}

func (client *Client) Stop() error {
	var err error
	client.stopOnce.Do(func() {
		err = client.server.Close()
	})
	return err
}
