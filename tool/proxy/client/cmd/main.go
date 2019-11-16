package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/kardianos/service"
	"github.com/pelletier/go-toml"

	"project/tools/proxy/client"
)

type program struct {
	client *client.Client
}

func (p *program) Start(_ service.Service) error {
	return p.client.Start()
}

func (p *program) Stop(_ service.Service) error {
	return p.client.Stop()
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
	var configs client.Configs
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

	pg := program{client: client.New(tag, &configs)}
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
