package server

import (
	"sync"

	"project/internal/crypto/cert"
	"project/internal/logger"
	"project/internal/proxy"
)

// Config contains proxy/server configurations.
type Config struct {
	Tag     string `toml:"-"`
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
}

// Server is proxy server.
type Server struct {
	network  string
	address  string
	server   *proxy.Server
	exitOnce sync.Once
}

// New is used to create a proxy server.
func New(config *Config) (*Server, error) {
	certPool, err := cert.NewPoolWithSystemCerts()
	if err != nil {
		return nil, err
	}
	manager := proxy.NewManager(certPool, logger.Common, nil)
	err = manager.Add(&proxy.Server{
		Tag:     config.Tag,
		Mode:    config.Proxy.Mode,
		Options: config.Proxy.Options,
	})
	if err != nil {
		return nil, err
	}
	proxyServer, _ := manager.Get(config.Tag)
	server := Server{
		network: config.Proxy.Network,
		address: config.Proxy.Address,
		server:  proxyServer,
	}
	return &server, nil
}

// Main is used to listen and server proxy server.
func (server *Server) Main() error {
	return server.server.ListenAndServe(server.network, server.address)
}

// Exit is used to close proxy server.
func (server *Server) Exit() (err error) {
	server.exitOnce.Do(func() {
		err = server.server.Close()
	})
	return
}

// Address is used to get proxy server address.
func (server *Server) Address() string {
	return server.server.Addresses()[0].String()
}
