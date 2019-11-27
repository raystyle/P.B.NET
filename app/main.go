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
	"github.com/pkg/errors"

	"project/controller"
)

func main() {
	var (
		debug     bool
		initDB    bool
		genKey    string
		install   bool
		uninstall bool
	)
	flag.BoolVar(&debug, "debug", false, "don't change current path")
	flag.BoolVar(&initDB, "initdb", false, "initialize database")
	flag.StringVar(&genKey, "genkey", "", "generate session key")
	flag.BoolVar(&install, "install", false, "install service")
	flag.BoolVar(&uninstall, "uninstall", false, "uninstall service")
	flag.Parse()

	if !debug {
		changePath()
	}
	config := loadConfig()
	pg := &program{config: config}
	svc, err := service.New(pg, &service.Config{
		Name:        "P.B.NET Controller",
		DisplayName: "P.B.NET Controller",
		Description: "P.B.NET Controller Service",
	})
	if err != nil {
		log.Fatal(err)
	}

	switch {
	case initDB:
		err = controller.InitializeDatabase(config)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to initialize database"))
		}
		log.Print("initialize database successfully")
	case genKey != "":
		err := controller.GenerateSessionKey(config.Global.KeyDir+"/session.key", genKey)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to generate session key"))
		}
		log.Print("generate controller keys successfully")
	case install:
		err = svc.Install()
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to install service"))
		}
		log.Print("install service successfully")
	case uninstall:
		err := svc.Uninstall()
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to uninstall service"))
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

func changePath() {
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

func loadConfig() *controller.Config {
	data, err := ioutil.ReadFile("config.toml")
	if err != nil {
		log.Fatal(err)
	}
	config := new(controller.Config)
	err = toml.Unmarshal(data, config)
	if err != nil {
		log.Fatal(err)
	}
	return config
}

type program struct {
	config   *controller.Config
	ctrl     *controller.CTRL
	stopOnce sync.Once
}

func (p *program) Start(s service.Service) error {
	var err error
	p.ctrl, err = controller.New(p.config)
	if err != nil {
		return err
	}
	go func() {
		err = p.ctrl.Main()
		if err != nil {
			l, e := s.Logger(nil)
			if e == nil {
				_ = l.Error(err)
			}
			os.Exit(1)
		}
		_ = s.Stop()
		os.Exit(0)
	}()
	return nil
}

func (p *program) Stop(_ service.Service) error {
	p.stopOnce.Do(func() {
		p.ctrl.Exit(nil)
	})
	return nil
}
