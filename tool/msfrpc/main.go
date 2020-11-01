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
	"sync"
	"time"

	"github.com/kardianos/service"
	"golang.org/x/crypto/bcrypt"

	"project/internal/logger"
	"project/internal/nettool"
	"project/internal/option"
	"project/internal/patch/toml"
	"project/internal/system"
	"project/internal/xpanic"
	"project/internal/xpprof"

	"project/msfrpc"
)

type config struct {
	Logger struct {
		Enable bool   `toml:"enable"`
		Level  string `toml:"level"`
		File   string `toml:"file"`
		Error  string `toml:"error"`
	} `toml:"logger"`

	Client struct {
		Address  string               `toml:"address"`
		Username string               `toml:"username"`
		Password string               `toml:"password"`
		Options  msfrpc.ClientOptions `toml:"options"`
	} `toml:"client"`

	Monitor msfrpc.MonitorOptions `toml:"monitor"`

	IOManager msfrpc.IOManagerOptions `toml:"io_manager"`

	Web struct {
		Network   string            `toml:"network"`
		Address   string            `toml:"address"`
		CertFile  string            `toml:"cert_file"`
		KeyFile   string            `toml:"key_file"`
		Directory string            `toml:"directory"`
		Options   msfrpc.WebOptions `toml:"options"`
	} `toml:"web"`

	PPROF struct {
		Enable   bool           `toml:"enable"`
		Network  string         `toml:"network"`
		Address  string         `toml:"address"`
		CertFile string         `toml:"cert_file"`
		KeyFile  string         `toml:"key_file"`
		Options  xpprof.Options `toml:"options"`
	} `toml:"pprof"`

	Service struct {
		Name        string `toml:"name"`
		DisplayName string `toml:"display_name"`
		Description string `toml:"description"`
	} `toml:"service"`
}

func main() {
	var (
		config    string
		install   bool
		uninstall bool
		genPass   string
		test      bool
	)
	flag.StringVar(&config, "config", "config.toml", "configuration file path")
	flag.BoolVar(&install, "install", false, "install service")
	flag.BoolVar(&uninstall, "uninstall", false, "uninstall service")
	flag.StringVar(&genPass, "gen", "", "generate password about web server")
	flag.BoolVar(&test, "test", false, "flag for test")
	flag.Parse()

	if genPass != "" {
		generateWebPassword(genPass)
		return
	}
	if !test {
		err := system.ChangeCurrentDirectory()
		if err != nil {
			log.Fatalln(err)
		}
	}

	// switch operation
	svc := createService(config)
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
		err := svc.Run()
		if err != nil {
			log.Fatalln(err)
		}
	}
}

// generateWebPassword is used to generate web server password.
func generateWebPassword(password string) {
	data, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println("password:", string(data))
}

func createService(cfg string) service.Service {
	// load msfrpc configuration
	data, err := ioutil.ReadFile(cfg) // #nosec
	if err != nil {
		log.Fatalln(err)
	}
	var config config
	err = toml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalln(err)
	}
	// set logger about initialize error
	logFile, err := logger.SetErrorLogger(config.Logger.Error)
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		_ = logFile.Sync()
		_ = logFile.Close()
	}()
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
	return svc
}

type program struct {
	logger  logger.Logger
	logFile *os.File

	msfrpc *msfrpc.MSFRPC

	// for pprof server
	pprof    *xpprof.Server
	listener net.Listener

	wg sync.WaitGroup
}

func newProgram(config *config) (*program, error) {
	// create logger
	logCfg := config.Logger
	var (
		mLogger logger.Logger
		logFile *os.File
	)
	if logCfg.Enable {
		level, err := logger.Parse(logCfg.Level)
		if err != nil {
			return nil, err
		}
		logFile, err := system.OpenFile(logCfg.File, os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			return nil, err
		}
		mLogger = logger.NewMultiLogger(level, os.Stdout, logFile)
	} else {
		mLogger = logger.Discard
	}

	// create MSFRPC configuration
	msfrpcCfg := msfrpc.Config{Logger: mLogger}

	clientCfg := config.Client
	msfrpcCfg.Client.Address = clientCfg.Address
	msfrpcCfg.Client.Username = clientCfg.Username
	msfrpcCfg.Client.Password = clientCfg.Password
	msfrpcCfg.Client.Options = &clientCfg.Options

	msfrpcCfg.Monitor = &config.Monitor
	msfrpcCfg.IOManager = &config.IOManager

	msfrpcCfg.Web.Network = config.Web.Network
	msfrpcCfg.Web.Address = config.Web.Address
	msfrpcCfg.Web.Options = &config.Web.Options

	// set server side tls certificate
	cert, err := ioutil.ReadFile(config.Web.CertFile)
	if err != nil {
		return nil, err
	}
	key, err := ioutil.ReadFile(config.Web.KeyFile)
	if err != nil {
		return nil, err
	}
	certs := msfrpcCfg.Web.Options.Server.TLSConfig.Certificates
	kp := option.X509KeyPair{
		Cert: string(cert),
		Key:  string(key),
	}
	certs = append([]option.X509KeyPair{kp}, certs...)
	msfrpcCfg.Web.Options.Server.TLSConfig.Certificates = certs
	// set web directory
	msfrpcCfg.Web.Options.HFS = http.Dir(config.Web.Directory)

	// create msfrpc
	MSFRPC, err := msfrpc.NewMSFRPC(&msfrpcCfg)
	if err != nil {
		return nil, err
	}
	program := program{
		logger:  mLogger,
		logFile: logFile,
		msfrpc:  MSFRPC,
	}
	// set pprof server
	pprof, err := newPPROFServer(mLogger, config)
	if err != nil {
		return nil, err
	}
	if pprof == nil {
		return &program, nil
	}
	listener, err := net.Listen(config.PPROF.Network, config.PPROF.Address)
	if err != nil {
		return nil, err
	}
	program.pprof = pprof
	program.listener = listener
	return &program, nil
}

func newPPROFServer(lg logger.Logger, config *config) (*xpprof.Server, error) {
	cfg := config.PPROF
	if !cfg.Enable {
		return nil, nil
	}
	cert, err := ioutil.ReadFile(config.PPROF.CertFile)
	if err != nil {
		return nil, err
	}
	key, err := ioutil.ReadFile(config.PPROF.KeyFile)
	if err != nil {
		return nil, err
	}
	opts := cfg.Options
	certs := opts.Server.TLSConfig.Certificates
	kp := option.X509KeyPair{
		Cert: string(cert),
		Key:  string(key),
	}
	certs = append([]option.X509KeyPair{kp}, certs...)
	opts.Server.TLSConfig.Certificates = certs
	return xpprof.NewHTTPSServer(lg, &opts)
}

func (p *program) log(lv logger.Level, log ...interface{}) {
	p.logger.Println(lv, "service", log...)
}

func (p *program) Start(service.Service) error {
	const title = "program.Start"

	logger.HijackLogWriter(logger.Error, "pkg", p.logger)
	errCh := make(chan error, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// start msfrpc
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				xpanic.Log(r, title)
			}
		}()
		errCh <- p.msfrpc.Main()
	}()
	_, err := nettool.WaitServerServe(ctx, errCh, p.msfrpc, 1)
	if err != nil {
		p.log(logger.Fatal, err)
		return err
	}
	p.msfrpc.Wait()

	// start pprof server
	if p.pprof == nil {
		return nil
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				xpanic.Log(r, title)
			}
		}()
		errCh <- p.pprof.Serve(p.listener)
	}()
	_, err = nettool.WaitServerServe(ctx, errCh, p.pprof, 1)
	if err != nil {
		p.log(logger.Fatal, err)
		return err
	}
	p.log(logger.Info, "pprof server is running")
	return nil
}

func (p *program) Stop(service.Service) error {
	// close msfrpc first
	p.msfrpc.Exit()
	// close pprof server
	if p.pprof != nil {
		err := p.pprof.Close()
		if err != nil {
			p.log(logger.Error, "appear error when close pprof server:", err)
		}
		p.log(logger.Info, "pprof server is closed")
	}
	p.wg.Wait()
	// close log file
	if p.logFile != nil {
		err := p.logFile.Close()
		if err != nil {
			p.log(logger.Error, "appear error when close log file:", err)
		}
	}
	p.log(logger.Info, "msfrpc service is stopped")
	return nil
}
