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
	c := &service.Config{
		Name:        controller.Name + " Controller",
		DisplayName: controller.Name + " Controller",
		Description: controller.Name + " Controller Service",
	}
	p := &program{}
	s, err := service.New(p, c)
	if err != nil {
		log.Fatal(err)
	}
	l, err := s.Logger(nil)
	if err != nil {
		log.Fatal(err)
	}
	err = s.Run()
	if err != nil {
		_ = l.Error(err)
	}
}

type program struct {
	ctrl *controller.CTRL
	once sync.Once
}

func (this *program) Start(s service.Service) error {
	var (
		debug     bool
		install   bool
		uninstall bool
		initdb    bool
		genkey    string
	)
	flag.BoolVar(&debug, "debug", false, "not changed path")
	flag.BoolVar(&install, "install", false, "install service")
	flag.BoolVar(&uninstall, "uninstall", false, "uninstall service")
	flag.BoolVar(&initdb, "initdb", false, "initialize database")
	flag.StringVar(&genkey, "genkey", "", "generate keys and encrypt it")
	flag.Parse()
	// install service
	if install {
		err := s.Install()
		if err != nil {
			return errors.Wrap(err, "install service failed")
		}
		log.Print("install service successfully")
		os.Exit(0)
	}
	// uninstall service
	if uninstall {
		err := s.Uninstall()
		if err != nil {
			return errors.Wrap(err, "uninstall service failed")
		}
		log.Print("uninstall service successfully")
		os.Exit(0)
	}
	// generate controller keys
	if genkey != "" {
		err := controller.Gen_CTRL_Keys(controller.Key_Path, genkey)
		if err != nil {
			return errors.Wrap(err, "generate keys failed")
		}
		log.Print("generate controller keys successfully")
		os.Exit(0)
	}
	// changed path for service
	if !debug {
		path, err := os.Executable()
		if err != nil {
			return err
		}
		path = strings.Replace(path, "\\", "/", -1) // windows
		err = os.Chdir(path[:strings.LastIndex(path, "/")])
		if err != nil {
			return err
		}
	}
	// load config
	data, err := ioutil.ReadFile("config.toml")
	if err != nil {
		return err
	}
	config := &controller.Config{}
	err = toml.Unmarshal(data, config)
	if err != nil {
		return err
	}
	// init database
	if initdb {
		err = controller.Init_Database(config)
		if err != nil {
			return errors.Wrap(err, "initialize database failed")
		}
		log.Print("initialize database successfully")
		os.Exit(0)
	}
	// run
	this.ctrl, err = controller.New(config)
	if err != nil {
		return err
	}
	go func() {
		err = this.ctrl.Main()
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

func (this *program) Stop(s service.Service) error {
	this.once.Do(func() {
		this.ctrl.Exit(nil)
	})
	return nil
}
