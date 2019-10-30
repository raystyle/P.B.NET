package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/kardianos/service"
	"github.com/pelletier/go-toml"

	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/proxy/socks"
)

type configs struct {
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

func main() {
	var (
		tag       string
		config    string
		debug     bool
		install   bool
		uninstall bool
	)
	flag.StringVar(&tag, "tag", "", "proxy client tag")
	flag.StringVar(&config, "config", "config.toml", "config file path")
	flag.BoolVar(&install, "install", false, "install service")
	flag.BoolVar(&uninstall, "uninstall", false, "uninstall service")
	flag.BoolVar(&debug, "debug", false, "don't change current path")
	flag.Parse()

	// changed path for service
	if !debug {
		path, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		path = strings.Replace(path, "\\", "/", -1) // windows
		err = os.Chdir(path[:strings.LastIndex(path, "/")])
		if err != nil {
			log.Fatal(err)
		}
	}

	// load config
	b, err := ioutil.ReadFile(config)
	if err != nil {
		log.Fatal(err)
	}
	var configs configs
	err = toml.Unmarshal(b, &configs)
	if err != nil {
		log.Fatal(err)
	}

	// start service
	svcCfg := service.Config{
		Name:        configs.Service.Name,
		DisplayName: configs.Service.DisplayName,
		Description: configs.Service.Description,
	}
	pg := program{
		tag:     tag,
		configs: &configs,
	}
	svc, err := service.New(&pg, &svcCfg)
	if err != nil {
		log.Fatal(err)
	}

	switch {
	case install:
		err = svc.Install()
		if err != nil {
			log.Fatalf("failed to install service: %s", err)
		}
		log.Print("install service successfully")
	case uninstall:
		err = svc.Uninstall()
		if err != nil {
			log.Fatalf("failed to uninstall service: %s", err)
		}
		log.Print("uninstall service successfully")
	default:
		lg, err := svc.Logger(nil)
		if err != nil {
			log.Fatal(err)
		}
		err = svc.Run()
		if err != nil {
			_ = lg.Error(err)
		}
	}
}

type program struct {
	tag      string
	configs  *configs
	server   *socks.Server
	stopOnce sync.Once
}

func (p *program) Start(s service.Service) error {
	pool := proxy.NewPool()
	for _, client := range p.configs.Clients {
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
	if p.tag == "" {
		p.tag = p.configs.Clients[len(p.configs.Clients)-1].Tag
	}

	client, err := pool.Get(p.tag)
	if err != nil {
		return err
	}

	// start socks5 server
	lConfig := p.configs.Listener
	opts := socks.Options{
		Username:    lConfig.Username,
		Password:    lConfig.Password,
		MaxConns:    lConfig.MaxConns,
		DialTimeout: client.DialTimeout,
	}
	p.server, err = socks.NewServer("mix", logger.Test, &opts)
	if err != nil {
		return err
	}
	return p.server.ListenAndServe(lConfig.Network, lConfig.Address)
}

func (p *program) Stop(_ service.Service) error {
	var err error
	p.stopOnce.Do(func() {
		err = p.server.Close()
	})
	return err
}
