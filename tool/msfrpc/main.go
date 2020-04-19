package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/kardianos/service"

	"project/internal/logger"
	"project/internal/patch/toml"

	"project/msfrpc"
)

type config struct {
	Logger struct {
		Level string `toml:"level"`
		File  string `toml:"file"`
	} `toml:"logger"`

	MSFRPC struct {
		Address  string `toml:"address"`
		Username string `toml:"username"`
		Password string `toml:"password"`
		msfrpc.Options
	} `toml:"msfrpc"`

	Database msfrpc.DBConnectOptions `toml:"database"`

	Web struct {
		Network   string `toml:"network"`
		Address   string `toml:"address"`
		Directory string `toml:"directory"`
		CertFile  string `toml:"cert_file"`
		KeyFile   string `toml:"key_file"`
		msfrpc.WebServerOptions
	} `toml:"web"`

	Service struct {
		Name        string `toml:"name"`
		DisplayName string `toml:"display_name"`
		Description string `toml:"description"`
	} `toml:"service"`
}

func main() {
	var (
		configPath string
		debug      bool
		install    bool
		uninstall  bool
	)
	flag.StringVar(&configPath, "config", "config.toml", "config file path")
	flag.BoolVar(&install, "install", false, "install service")
	flag.BoolVar(&uninstall, "uninstall", false, "uninstall service")
	flag.BoolVar(&debug, "debug", false, "don't change current path")
	flag.Parse()

	// changed path for service
	if !debug {
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

	// load msfrpc config
	data, err := ioutil.ReadFile(configPath) // #nosec
	if err != nil {
		log.Fatalln(err)
	}
	var config config
	err = toml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalln(err)
	}

	// initialize service
	program := createProgram(&config)
	svcConfig := service.Config{
		Name:        config.Service.Name,
		DisplayName: config.Service.DisplayName,
		Description: config.Service.Description,
	}
	svc, err := service.New(program, &svcConfig)
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

func createProgram(config *config) *program {
	// create logger
	logCfg := config.Logger
	level, err := logger.Parse(logCfg.Level)
	if err != nil {
		log.Fatalln(err)
	}
	logFile, err := os.OpenFile(logCfg.File, os.O_CREATE|os.O_APPEND, 0600) // #nosec
	if err != nil {
		log.Fatalln(err)
	}
	mLogger := logger.NewMultiLogger(level, logFile)
	// create MSFRPC
	address := config.MSFRPC.Address
	username := config.MSFRPC.Username
	password := config.MSFRPC.Password
	options := config.MSFRPC.Options
	MSFRPC, err := msfrpc.NewMSFRPC(address, username, password, mLogger, &options)
	if err != nil {
		log.Fatalln(err)
	}
	return &program{
		log:    logFile,
		msfrpc: MSFRPC,
	}
}

type program struct {
	log    *os.File
	config *config

	msfrpc    *msfrpc.MSFRPC
	webServer *msfrpc.WebServer
	monitor   *msfrpc.Monitor

	wg sync.WaitGroup
}

func (p *program) Start(s service.Service) error {
	// login
	if p.msfrpc.GetToken() == "" {
		err := p.msfrpc.AuthLogin()
		if err != nil {
			return err
		}
	}
	// connect database
	err := p.msfrpc.DBConnect(context.Background(), &p.config.Database)
	if err != nil {
		return err
	}

	// start web server
	// webServer, err := p.msfrpc.NewWebServer()
	// if err != nil {
	// 	return err
	// }

	// p.msfrpc.NewMonitor()

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		// err := p.server.Main()
		// if err != nil {
		// 	l, e := s.Logger(nil)
		// 	if e == nil {
		// 		_ = l.Error(err)
		// 	}
		// 	os.Exit(1)
		// }
	}()
	return nil
}

func (p *program) Stop(_ service.Service) error {
	// err := p.server.Exit()
	p.wg.Wait()
	return nil
}
