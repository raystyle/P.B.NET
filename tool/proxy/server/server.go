package server

import (
	"sync"

	"project/internal/logger"
	"project/internal/proxy"
)

// Configs contains proxy/server configurations
type Configs struct {
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

	Tag string // for test
}

// Server is proxy server
type Server struct {
	configs  *Configs
	server   *proxy.Server
	stopOnce sync.Once
}

// New is used to create a proxy server
func New(config *Configs) *Server {
	return &Server{configs: config}
}

// Main is used to run program
func (server *Server) Main() error {
	manager := proxy.NewManager(logger.Common, nil)
	srv := &proxy.Server{
		Tag:     server.configs.Tag,
		Mode:    server.configs.Proxy.Mode,
		Options: server.configs.Proxy.Options,
	}
	err := manager.Add(srv)
	if err != nil {
		return err
	}
	server.server = srv
	network := server.configs.Proxy.Network
	address := server.configs.Proxy.Address
	return srv.ListenAndServe(network, address)
}

// Exit is used to exit program
func (server *Server) Exit() error {
	var err error
	server.stopOnce.Do(func() {
		err = server.server.Close()
	})
	return err
}

// Address is used to get proxy server address
func (server *Server) Address() string {
	return server.server.Addresses()[0].String()
}
