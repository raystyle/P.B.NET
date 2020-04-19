package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"

	"project/internal/patch/toml"

	"project/msfrpc"
)

type config struct {
	MSFRPC struct {
		Address  string `toml:"address"`
		Username string `toml:"username"`
		Password string `toml:"password"`
		msfrpc.Options
	} `toml:"msfrpc"`

	Database *msfrpc.DBConnectOptions `toml:"database"`

	Web struct {
		Address string `toml:"address"`
		msfrpc.WebOptions
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

	// MSFRPC, err := msfrpc.NewMSFRPC(config.MSFRPC.Address)

	// initialize service

	_, _ = msfrpc.NewMSFRPC("", "", "", nil, nil)
}

type program struct {
	msfrpc  *msfrpc.MSFRPC
	monitor *msfrpc.Monitor
	web     *msfrpc.WebServer
	wg      sync.WaitGroup
}
