package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/kardianos/service"
	"github.com/pkg/errors"

	"project/internal/patch/toml"
	"project/internal/system"

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

	if initDB {
		err := controller.InitializeDatabase(loadConfig())
		if err != nil {
			log.Fatalln("failed to initialize database:", err)
		}
		log.Println("initialize database successfully")
		return
	}

	if genKey != "" {
		err := generateSessionKey([]byte(genKey))
		if err != nil {
			log.Fatalln("failed to generate session key:", err)
		}
		log.Println("generate controller keys successfully")
		return
	}

	svc := createService()
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

func changePath() {
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

func loadConfig() *controller.Config {
	data, err := ioutil.ReadFile("config.toml")
	if err != nil {
		log.Fatalln(err)
	}
	config := new(controller.Config)
	err = toml.Unmarshal(data, config)
	if err != nil {
		log.Fatalln(err)
	}
	return config
}

func generateSessionKey(password []byte) error {
	_, err := os.Stat(controller.SessionKeyFile)
	if !os.IsNotExist(err) {
		return errors.Errorf("file: %s already exist", controller.SessionKeyFile)
	}
	key, err := controller.GenerateSessionKey(password)
	if err != nil {
		return nil
	}
	return system.WriteFile(controller.SessionKeyFile, key)
}

func createService() service.Service {
	ctrl, err := controller.New(loadConfig())
	if err != nil {
		log.Fatalln(err)
	}
	ctrl.HijackLogWriter()
	svc, err := service.New(&program{ctrl: ctrl}, &service.Config{
		Name:        "P.B.NET Controller",
		DisplayName: "P.B.NET Controller",
		Description: "P.B.NET Controller Service",
	})
	if err != nil {
		log.Fatalln(err)
	}
	return svc
}

type program struct {
	ctrl *controller.Ctrl
	wg   sync.WaitGroup
}

func (p *program) Start(s service.Service) error {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		err := p.ctrl.Main()
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

func (p *program) Stop(_ service.Service) error {
	p.ctrl.Exit(nil)
	p.wg.Wait()
	return nil
}
