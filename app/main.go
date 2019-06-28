package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/kardianos/service"
	"github.com/pelletier/go-toml"

	"project/controller"
)

var (
	debug     bool
	install   bool
	uninstall bool
	genkey    string
	initdb    bool
)

func main() {
	config := &service.Config{
		Name:        controller.Name + " Controller",
		DisplayName: controller.Name + " Controller",
		Description: controller.Name + " Controller Service",
	}
	p := &program{}
	s, err := service.New(p, config)
	if err != nil {
		log.Fatal(err)
	}
	flag.BoolVar(&debug, "debug", false, "not changed path")
	flag.BoolVar(&install, "install", false, "install service")
	flag.BoolVar(&uninstall, "uninstall", false, "uninstall service")
	flag.StringVar(&genkey, "genkey", "", "generate keys and encrypt it")
	flag.BoolVar(&initdb, "initdb", false, "initialize database")
	flag.Parse()
	if install {
		err = s.Install()
		if err != nil {
			log.Fatal("install service failed: ", err)
		}
		log.Print("install service successfully.")
		return
	}
	if uninstall {
		err = s.Uninstall()
		if err != nil {
			log.Fatal("uninstall service failed: ", err)
		}
		log.Print("uninstall service successfully.")
		return
	}
	// generate controller keys
	if genkey != "" {
		err = controller.Gen_CTRL_Keys(genkey)
		if err != nil {
			log.Fatal(err)
		}
		log.Print("generate controller keys successfully.")
		return
	}
	logger, err := s.Logger(nil)
	if err != nil {
		log.Fatal(err)
	}
	err = s.Run()
	if err != nil {
		_ = logger.Error(err)
	}
}

type program struct {
	ctrl *controller.CTRL
}

func (this *program) Start(s service.Service) error {
	// changed path
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
	ctrl, err := controller.New(config)
	if err != nil {
		return err
	}
	err = ctrl.Main()
	if err != nil {
		return err
	}
	this.ctrl = ctrl
	return nil
}

func (this *program) Stop(s service.Service) error {
	this.ctrl.Exit()
	return nil
}
