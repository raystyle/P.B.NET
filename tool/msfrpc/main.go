package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kardianos/service"

	"project/internal/logger"
	"project/internal/option"
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

	WebServer struct {
		Network   string            `toml:"network"`
		Address   string            `toml:"address"`
		Username  string            `toml:"username"`
		Password  string            `toml:"password"`
		Directory string            `toml:"directory"`
		CertFile  string            `toml:"cert_file"`
		KeyFile   string            `toml:"key_file"`
		MaxConns  int               `toml:"max_conns"`
		Options   option.HTTPServer `toml:"options"`
	} `toml:"web_server"`

	Advance struct {
		IOInterval      time.Duration `toml:"io_interval"`
		MonitorInterval time.Duration `toml:"monitor_interval"`
	} `toml:"advance"`

	Service struct {
		Name        string `toml:"name"`
		DisplayName string `toml:"display_name"`
		Description string `toml:"description"`
	} `toml:"service"`
}

func main() {
	var (
		password  string
		test      bool
		cfgPath   string
		install   bool
		uninstall bool
	)
	flag.StringVar(&password, "pass", "", "generate password about web server")
	flag.BoolVar(&test, "test", false, "don't change current path")
	flag.StringVar(&cfgPath, "config", "config.toml", "configuration file path")
	flag.BoolVar(&install, "install", false, "install service")
	flag.BoolVar(&uninstall, "uninstall", false, "uninstall service")
	flag.Parse()

	// generate password
	// if password != "" {
	// 	data, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	// 	if err != nil {
	// 		log.Fatalln(err)
	// 	}
	// 	fmt.Println("password:", string(data))
	// 	return
	// }

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

	// load msfrpc configuration
	data, err := ioutil.ReadFile(cfgPath) // #nosec
	if err != nil {
		log.Fatalln(err)
	}
	var config config
	err = toml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalln(err)
	}

	// initialize service
	program, err := newProgram(&config)
	if err != nil {
		log.Fatalln(err)
	}
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

type program struct {
	config *config

	log       *os.File
	msfrpc    *msfrpc.MSFRPC
	webServer *msfrpc.WebServer
	monitor   *msfrpc.Monitor

	wg sync.WaitGroup
}

func newProgram(config *config) (*program, error) {
	// create logger
	logCfg := config.Logger
	level, err := logger.Parse(logCfg.Level)
	if err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(logCfg.File, os.O_CREATE|os.O_APPEND, 0600) // #nosec
	if err != nil {
		return nil, err
	}
	mLogger := logger.NewMultiLogger(level, os.Stdout, logFile)
	// create MSFRPC
	address := config.MSFRPC.Address
	username := config.MSFRPC.Username
	password := config.MSFRPC.Password
	options := config.MSFRPC.Options
	MSFRPC, err := msfrpc.NewMSFRPC(address, username, password, mLogger, &options)
	if err != nil {
		return nil, err
	}
	return &program{
		config: config,
		log:    logFile,
		msfrpc: MSFRPC,
	}, nil
}

func (p *program) Main() error {
	return nil
}

func (p *program) Exit() error {
	return nil
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
	// start listener
	webCfg := p.config.WebServer
	lAddr, err := net.ResolveTCPAddr(webCfg.Network, webCfg.Address)
	if err != nil {
		return err
	}
	listener, err := net.ListenTCP(webCfg.Network, lAddr)
	if err != nil {
		return err
	}
	// set server side tls certificate
	cert, err := ioutil.ReadFile(webCfg.CertFile)
	if err != nil {
		return err
	}
	key, err := ioutil.ReadFile(webCfg.KeyFile)
	if err != nil {
		return err
	}
	certs := webCfg.Options.TLSConfig.Certificates
	certs = append([]option.X509KeyPair{{string(cert), string(key)}}, certs...)
	webCfg.Options.TLSConfig.Certificates = certs

	// set advanced options
	// 	webCfg.WebServerOptions.IOInterval = p.config.Advance.IOInterval
	// web file directory
	// fs := http.Dir(webCfg.Directory)
	// start web server
	// p.webServer, err = p.msfrpc.NewWebServer(fs, &webCfg.WebServerOptions)
	// if err != nil {
	// 	return err
	// }
	// start monitor
	p.monitor = p.msfrpc.NewMonitor(nil, p.config.Advance.MonitorInterval)

	fmt.Println("ok")

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		_ = listener.Close()

		// _ = p.webServer.Serve(listener)

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
