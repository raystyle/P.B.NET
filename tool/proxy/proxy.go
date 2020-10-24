package proxy

import (
	"project/internal/cert"
	"project/internal/logger"
	"project/internal/proxy"
)

// ClientConfig contains internal/proxy/proxy.go: Client configurations.
type ClientConfig struct {
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

// Client is a proxy client with a proxy server(front end).
type Client struct {
	network string
	address string
	server  *proxy.Server
}

// NewClient is used to create a proxy client.
func NewClient(config *ClientConfig) (*Client, error) {
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
		Tag:         proxy.EmptyTag,
		Mode:        server.Mode,
		Options:     server.Options,
		DialContext: proxyClient.DialContext,
	})
	if err != nil {
		return nil, err
	}
	proxyServer, _ := manager.Get(proxy.EmptyTag)
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
	return client.server.Close()
}

// testAddress is used to get proxy server address.
func (client *Client) testAddress() string {
	return client.server.Addresses()[0].String()
}

// ServerConfig contains internal/proxy/proxy.go: Server configurations.
type ServerConfig struct {
	Service struct {
		Name        string `toml:"name"`
		DisplayName string `toml:"display_name"`
		Description string `toml:"description"`
	} `toml:"service"`

	Proxy struct {
		Mode    string `toml:"mode"`
		Network string `toml:"network"`
		Address string `toml:"address"`
		Options string `toml:"options"`
	} `toml:"proxy"`

	// for test
	tag string `toml:"-"`
}

// Server is a proxy server.
type Server struct {
	network string
	address string
	server  *proxy.Server
}

// NewServer is used to create a proxy server.
func NewServer(config *ServerConfig) (*Server, error) {
	certPool, err := cert.NewPoolWithSystemCerts()
	if err != nil {
		return nil, err
	}
	manager := proxy.NewManager(certPool, logger.Common, nil)
	if config.tag == "" {
		config.tag = proxy.EmptyTag
	}
	err = manager.Add(&proxy.Server{
		Tag:     config.tag,
		Mode:    config.Proxy.Mode,
		Options: config.Proxy.Options,
	})
	if err != nil {
		return nil, err
	}
	proxyServer, _ := manager.Get(config.tag)
	server := Server{
		network: config.Proxy.Network,
		address: config.Proxy.Address,
		server:  proxyServer,
	}
	return &server, nil
}

// Main is used to listen and server proxy server.
func (srv *Server) Main() error {
	return srv.server.ListenAndServe(srv.network, srv.address)
}

// Exit is used to close proxy server.
func (srv *Server) Exit() error {
	return srv.server.Close()
}

// testAddress is used to get proxy server address.
func (srv *Server) testAddress() string {
	return srv.server.Addresses()[0].String()
}
