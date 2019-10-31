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
)

type configs struct {
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

func main() {
	var (
		config    string
		debug     bool
		install   bool
		uninstall bool
	)
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
	pg := program{configs: &configs}
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
	configs  *configs
	manager  *proxy.Manager
	stopOnce sync.Once
}

func (p *program) Start(s service.Service) error {
	const tag = "server"
	p.manager = proxy.NewManager(logger.Test, nil)
	err := p.manager.Add(&proxy.Server{
		Tag:     tag,
		Mode:    p.configs.Proxy.Mode,
		Options: p.configs.Proxy.Options,
	})
	if err != nil {
		return err
	}
	ps, _ := p.manager.Get(tag)
	network := p.configs.Proxy.Network
	address := p.configs.Proxy.Address
	return ps.ListenAndServe(network, address)
}

func (p *program) Stop(_ service.Service) error {
	var err error
	p.stopOnce.Do(func() {
		err = p.manager.Close()
	})
	return err
}
