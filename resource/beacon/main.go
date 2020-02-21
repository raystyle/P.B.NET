package main

import (
	"bytes"
	"flag"
	"log"
	"os"
	"sync"
	"time"

	"github.com/kardianos/service"

	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/crypto/ed25519"
	"project/internal/patch/msgpack"

	"project/beacon"
)

func main() {
	var (
		install   bool
		uninstall bool
	)
	flag.BoolVar(&install, "install", false, "install service")
	flag.BoolVar(&uninstall, "uninstall", false, "uninstall service")
	flag.Parse()

	config := new(beacon.Config)
	err := msgpack.Unmarshal([]byte{}, config)
	if err != nil {
		log.Fatalln(err)
	}

	// TODO remove
	tempSetConfig(config)

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

func createService(config *beacon.Config) service.Service {
	Beacon, err := beacon.New(config)
	if err != nil {
		log.Fatalln(err)
	}
	Beacon.HijackLogWriter()
	svc, err := service.New(&program{beacon: Beacon}, &service.Config{
		Name:        config.Service.Name,
		DisplayName: config.Service.DisplayName,
		Description: config.Service.Description,
	})
	if err != nil {
		log.Fatalln(err)
	}
	return svc
}

type program struct {
	beacon *beacon.Beacon
	wg     sync.WaitGroup
}

func (p *program) Start(s service.Service) error {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		err := p.beacon.Main()
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
	p.beacon.Exit(nil)
	p.wg.Wait()
	return nil
}

func tempSetConfig(config *beacon.Config) {
	config.Test.SkipSynchronizeTime = true

	config.Logger.Level = "debug"
	config.Logger.QueueSize = 512
	config.Logger.Writer = os.Stdout

	config.Global.DNSCacheExpire = 3 * time.Minute
	config.Global.TimeSyncInterval = 1 * time.Minute
	// config.Global.Certificates = testdata.Certificates(tb)
	// config.Global.ProxyClients = testdata.ProxyClients(tb)
	// config.Global.DNSServers = testdata.DNSServers()
	// config.Global.TimeSyncerClients = testdata.TimeSyncerClients(tb)

	config.Client.ProxyTag = "balance"
	config.Client.Timeout = 15 * time.Second

	config.Sender.Worker = 64
	config.Sender.QueueSize = 512
	config.Sender.MaxBufferSize = 512 << 10
	config.Sender.Timeout = 15 * time.Second

	config.Syncer.ExpireTime = 30 * time.Second

	config.Worker.Number = 16
	config.Worker.QueueSize = 1024
	config.Worker.MaxBufferSize = 16384

	config.Ctrl.KexPublicKey = bytes.Repeat([]byte{255}, curve25519.ScalarSize)
	config.Ctrl.PublicKey = bytes.Repeat([]byte{255}, ed25519.PublicKeySize)
	config.Ctrl.BroadcastKey = bytes.Repeat([]byte{255}, aes.Key256Bit+aes.IVSize)
}
