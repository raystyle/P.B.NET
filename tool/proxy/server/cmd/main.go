package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/kardianos/service"

	"project/internal/patch/toml"

	"project/tool/proxy/server"
)

func main() {
	var (
		test      bool
		config    string
		install   bool
		uninstall bool
	)
	flag.BoolVar(&test, "test", false, "don't change current path")
	flag.StringVar(&config, "config", "config.toml", "configuration file path")
	flag.BoolVar(&install, "install", false, "install service")
	flag.BoolVar(&uninstall, "uninstall", false, "uninstall service")
	flag.Parse()

	// changed path for service and prevent get invalid path when test
	if !test {
		path, err := os.Executable()
		if err != nil {
			log.Fatalln(err)
		}
		dir, _ := filepath.Split(path)
		err = os.Chdir(dir)
		if err != nil {
			log.Fatalln(err)
		}
	}

	// load proxy server configuration
	data, err := ioutil.ReadFile(config) // #nosec
	if err != nil {
		log.Fatalln(err)
	}
	var configs server.Config
	err = toml.Unmarshal(data, &configs)
	if err != nil {
		log.Fatalln(err)
	}
	configs.Tag = "server"
	proxyServer, err := server.New(&configs)
	if err != nil {
		log.Fatalln(err)
	}

	// initialize service
	program := program{server: proxyServer}
	svcConfig := service.Config{
		Name:        configs.Service.Name,
		DisplayName: configs.Service.DisplayName,
		Description: configs.Service.Description,
	}
	svc, err := service.New(&program, &svcConfig)
	if err != nil {
		log.Fatalln(err)
	}

	// switch operation
	switch {
	case install:
		err = svc.Install()
		if err != nil {
			log.Fatalln("failed to install service:", err)
		}
		log.Println("install service successfully")
	case uninstall:
		err = svc.Uninstall()
		if err != nil {
			log.Fatalln("failed to uninstall service:", err)
		}
		log.Println("uninstall service successfully")
	default:
		lg, err := svc.Logger(nil)
		if err != nil {
			log.Fatalln(err)
		}
		err = svc.Run()
		if err != nil {
			_ = lg.Error(err)
		}
	}
}

type program struct {
	server *server.Server
	wg     sync.WaitGroup
}

func (p *program) Start(s service.Service) error {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		err := p.server.Main()
		if err != nil {
			l, e := s.Logger(nil)
			if e == nil {
				_ = l.Error(err)
			}
			os.Exit(1)
		}
	}()
	return nil
}

func (p *program) Stop(service.Service) error {
	err := p.server.Exit()
	p.wg.Wait()
	return err
}
