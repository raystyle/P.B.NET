package server

import (
	"sync"

	"project/internal/logger"
	"project/internal/proxy"
)

// Configs contains proxy/server configurations
type Configs struct {
	Tag     string `toml:"tag"`
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
	tag := server.configs.Tag
	manager := proxy.NewManager(logger.Test, nil)
	err := manager.Add(&proxy.Server{
		Tag:     tag,
		Mode:    server.configs.Proxy.Mode,
		Options: server.configs.Proxy.Options,
	})
	if err != nil {
		return err
	}
	ps, _ := manager.Get(tag)
	server.server = ps
	network := server.configs.Proxy.Network
	address := server.configs.Proxy.Address
	return ps.ListenAndServe(network, address)
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
	return server.server.Address()
}
