package client

import (
	"sync"

	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/proxy/socks"
)

// Configs contains proxy/client configurations
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

	Clients []*proxy.Client `toml:"clients"`
}

// Client is proxy client
type Client struct {
	tag     string
	configs *Configs
	server  *socks.Server
	exit    sync.Once
}

// New is used to create a proxy client
func New(tag string, config *Configs) *Client {
	return &Client{tag: tag, configs: config}
}

// Main is used to run program
func (client *Client) Main() error {
	pool := proxy.NewPool()
	for _, client := range client.configs.Clients {
		err := pool.Add(client)
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
		DialContext: pc.DialContext,
	}
	client.server, err = socks.NewSocks5Server("proxy", logger.Common, &opts)
	if err != nil {
		return err
	}
	return client.server.ListenAndServe(lc.Network, lc.Address)
}

// Exit is used to exit program
func (client *Client) Exit() error {
	var err error
	client.exit.Do(func() {
		err = client.server.Close()
	})
	return err
}

// Address is used to get socks5 server address
func (client *Client) Address() string {
	return client.server.Addresses()[0].String()
}
