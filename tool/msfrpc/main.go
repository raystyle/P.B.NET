package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kardianos/service"
	"golang.org/x/crypto/bcrypt"

	"project/internal/logger"
	"project/internal/option"
	"project/internal/patch/toml"
	"project/internal/xpanic"

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
		Options   option.HTTPServer `toml:"options" check:"-"`
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

	if password != "" {
		generateWebServerPassword(password)
		return
	}

	if !test {
		changeCurrentDirectory()
	}

	setErrorLogger()

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

	// initialize program
	program, err := newProgram(&config)
	if err != nil {
		log.Fatalln(err)
	}

	// initialize service
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

// generateWebServerPassword is used to generate web server password.
func generateWebServerPassword(password string) {
	data, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println("password:", string(data))
}

// changeCurrentDirectory is used to changed path for service and prevent
// get invalid path when test.
func changeCurrentDirectory() {
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

// setErrorLogger is used to log error before program start.
func setErrorLogger() {
	file, err := os.OpenFile("msfrpc.err", os.O_CREATE|os.O_APPEND, 0600) // #nosec
	if err != nil {
		log.Fatalln(err)
	}
	mLogger := logger.NewMultiLogger(logger.Error, os.Stdout, file)
	logger.HijackLogWriter(logger.Error, "init", mLogger, 0)
}

type program struct {
	config *config

	log       *os.File
	listener  net.Listener
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

	// start listener for http server
	webCfg := config.WebServer
	lAddr, err := net.ResolveTCPAddr(webCfg.Network, webCfg.Address)
	if err != nil {
		return nil, err
	}
	listener, err := net.ListenTCP(webCfg.Network, lAddr)
	if err != nil {
		return nil, err
	}

	// set server side tls certificate
	cert, err := ioutil.ReadFile(webCfg.CertFile)
	if err != nil {
		return nil, err
	}
	key, err := ioutil.ReadFile(webCfg.KeyFile)
	if err != nil {
		return nil, err
	}
	certs := webCfg.Options.TLSConfig.Certificates
	kp := option.X509KeyPair{
		Cert: string(cert),
		Key:  string(key),
	}
	certs = append([]option.X509KeyPair{kp}, certs...)
	webCfg.Options.TLSConfig.Certificates = certs

	// create web server
	webOpts := msfrpc.WebServerOptions{
		HTTPServer: webCfg.Options,
		MaxConns:   webCfg.MaxConns,
		IOInterval: config.Advance.IOInterval,
	}
	fs := http.Dir(webCfg.Directory)
	webServer, err := MSFRPC.NewWebServer(webCfg.Username, webCfg.Password, fs, &webOpts)
	if err != nil {
		return nil, err
	}
	return &program{
		config:    config,
		log:       logFile,
		listener:  listener,
		msfrpc:    MSFRPC,
		webServer: webServer,
	}, nil
}

func (p *program) Start(s service.Service) error {
	// login
	token := p.msfrpc.GetToken()
	if token == "" {
		err := p.msfrpc.AuthLogin()
		if err != nil {
			return err
		}
	}
	// connect database
	err := p.msfrpc.DBConnect(context.Background(), &p.config.Database)
	if err != nil {
		_ = p.msfrpc.AuthLogout(token)
		return err
	}
	// start monitor
	callbacks := p.webServer.Callbacks()
	interval := p.config.Advance.MonitorInterval
	p.monitor = p.msfrpc.NewMonitor(callbacks, interval)
	p.monitor.StartDatabaseMonitors()
	// start web server
	p.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println(xpanic.Print(r, "program.Start"))
			}
			p.wg.Done()
		}()
		err := p.webServer.Serve(p.listener)
		if err != nil && err != http.ErrServerClosed {
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
	_ = p.webServer.Close()
	p.wg.Wait()
	p.monitor.Close()
	_ = p.msfrpc.Close()
	_ = p.log.Close()
	return nil
}
