package main

import (
	"flag"
	"log"
	"os"
	"sync"

	"github.com/kardianos/service"

	"project/node"
)

func main() {
	var (
		install   bool
		uninstall bool
	)
	flag.BoolVar(&install, "install", false, "install service")
	flag.BoolVar(&uninstall, "uninstall", false, "uninstall service")
	flag.Parse()

	// generator will replace it
	data := []byte("flag-01: config")
	key := []byte("flag-02: key")
	var config node.Config
	err := config.Load(data, key)
	if err != nil {
		log.Fatalln(err)
	}

	svc := createService(&config)
	switch {
	case install:
		err := svc.Install()
		if err != nil {
			log.Fatalln("failed to install service:", err)
		}
		log.Println("install service successfully")
	case uninstall:
		err := svc.Uninstall()
		if err != nil {
			log.Fatalln("failed to uninstall service:", err)
		}
		log.Println("uninstall service successfully")
	default:
		err = svc.Run()
		if err != nil {
			log.Println(err)
		}
	}
}

func createService(config *node.Config) service.Service {
	Node, err := node.New(config)
	if err != nil {
		log.Fatalln(err)
	}
	svc, err := service.New(&program{node: Node}, &service.Config{
		Name:        config.Service.Name,
		DisplayName: config.Service.DisplayName,
		Description: config.Service.Description,
	})
	if err != nil {
		log.Fatalln(err)
	}
	Node.HijackLogWriter()
	return svc
}

type program struct {
	node *node.Node
	wg   sync.WaitGroup
}

func (p *program) Start(service.Service) error {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		err := p.node.Main()
		if err != nil {
			os.Exit(1)
		}
	}()
	return nil
}

func (p *program) Stop(service.Service) error {
	p.node.Exit(nil)
	p.wg.Wait()
	return nil
}
