package client

import (
	"sync"

	"project/internal/crypto/cert"
	"project/internal/logger"
	"project/internal/proxy"
)

// Config contains proxy/client configurations.
type Config struct {
	// service config
	Service struct {
		Name        string `toml:"name"`
		DisplayName string `toml:"display_name"`
		Description string `toml:"description"`
	} `toml:"service"`

	// front proxy server config
	Server struct {
		Mode    string `toml:"mode"`
		Network string `toml:"network"`
		Address string `toml:"address"`
		Options string `toml:"options"`
	} `toml:"server"`

	// proxy clients
	Tag     string          `toml:"tag"`
	Clients []*proxy.Client `toml:"clients"`
}

// Client is a proxy client with a proxy server.
type Client struct {
	network  string
	address  string
	server   *proxy.Server
	exitOnce sync.Once
}

// New is used to create a proxy client.
func New(config *Config) (*Client, error) {
	certPool, err := cert.NewPoolWithSystemCerts()
	if err != nil {
		return nil, err
	}
	pool := proxy.NewPool(certPool)
	for _, client := range config.Clients {
		err := pool.Add(client)
		if err != nil {
			return nil, err
		}
	}
	// if tag, use the last proxy client
	tag := config.Tag
	if tag == "" {
		tag = config.Clients[len(config.Clients)-1].Tag
	}
	// set proxy client
	proxyClient, err := pool.Get(tag)
	if err != nil {
		return nil, err
	}
	// start front proxy server
	server := config.Server
	manager := proxy.NewManager(certPool, logger.Common, nil)
	err = manager.Add(&proxy.Server{
		Tag:         "proxy",
		Mode:        server.Mode,
		Options:     server.Options,
		DialContext: proxyClient.DialContext,
	})
	if err != nil {
		return nil, err
	}
	proxyServer, _ := manager.Get("proxy")
	client := Client{
		network: server.Network,
		address: server.Address,
		server:  proxyServer,
	}
	return &client, nil
}

// Main is used to listen and server front proxy server.
func (client *Client) Main() error {
	return client.server.ListenAndServe(client.network, client.address)
}

// Exit is used to close front proxy server.
func (client *Client) Exit() error {
	var err error
	client.exitOnce.Do(func() {
		err = client.server.Close()
	})
	return err
}

// Address is used to get proxy server address.
func (client *Client) Address() string {
	return client.server.Addresses()[0].String()
}
