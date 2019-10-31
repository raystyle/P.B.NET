package server

import (
	"sync"

	"project/internal/logger"
	"project/internal/proxy"
)

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
}

type Server struct {
	configs  *Configs
	server   *proxy.Server
	stopOnce sync.Once
}

func New(config *Configs) *Server {
	return &Server{configs: config}
}

func (server *Server) Start() error {
	const tag = "server"
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

func (server *Server) Stop() error {
	var err error
	server.stopOnce.Do(func() {
		err = server.server.Close()
	})
	return err
}

func (server *Server) Address() string {
	return server.server.Address()
}
