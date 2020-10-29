package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/kardianos/service"
	"golang.org/x/crypto/bcrypt"

	"project/internal/logger"
	"project/internal/option"
	"project/internal/patch/toml"
	"project/internal/system"
	"project/internal/xpanic"

	"project/msfrpc"
)

type config struct {
	Logger struct {
		Level string `toml:"level"`
		File  string `toml:"file"`
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
	flag.BoolVar(&test, "test", false, "a flag for test")
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
	logFile, err := logger.SetErrorLogger("msfrpc.err")
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		_ = logFile.Sync()
		_ = logFile.Close()
	}()

	// switch operation
	svc := createService(config)
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
		err = svc.Run()
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
	logFile *os.File
	logger  logger.Logger
	msfrpc  *msfrpc.MSFRPC
	wg      sync.WaitGroup
}

func newProgram(config *config) (*program, error) {
	// create logger
	logCfg := config.Logger
	level, err := logger.Parse(logCfg.Level)
	if err != nil {
		return nil, err
	}
	logFile, err := system.OpenFile(logCfg.File, os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	mLogger := logger.NewMultiLogger(level, os.Stdout, logFile)

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

	MSFRPC, err := msfrpc.NewMSFRPC(&msfrpcCfg)
	if err != nil {
		return nil, err
	}
	return &program{
		logFile: logFile,
		logger:  mLogger,
		msfrpc:  MSFRPC,
	}, nil
}

func (p *program) Start(service.Service) error {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				xpanic.Log(r, "program.Start")
			}
		}()
		p.msfrpc.HijackLogWriter()
		err := p.msfrpc.Main()
		if err != nil {
			p.logger.Print(logger.Fatal, "service", err)
			os.Exit(1)
		}
	}()
	return nil
}

func (p *program) Stop(service.Service) error {
	p.msfrpc.Exit()
	p.wg.Wait()
	_ = p.logFile.Close()
	return nil
}
