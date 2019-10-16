package controller

import (
	"sync"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	"project/internal/dns"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/timesync"
)

type CTRL struct {
	Debug *Debug

	db      *db      // database
	logger  *xLogger // logger
	global  *global  // proxy, dns, time syncer, and ...
	handler *handler // handle message from Node or Beacon
	sender  *sender  // broadcast and send message
	syncer  *syncer  // receive message
	boot    *boot    // auto discover bootstrap nodes
	web     *web     // web server

	once sync.Once
	wait chan struct{}
	exit chan error
}

func New(cfg *Config) (*CTRL, error) {
	// database
	db, err := newDB(cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "initialize database failed")
	}
	// copy debug config
	debug := cfg.Debug
	ctrl := &CTRL{
		Debug: &debug,
		db:    db,
	}
	// logger
	lg, err := newLogger(ctrl, cfg.LogLevel)
	if err != nil {
		return nil, errors.WithMessage(err, "initialize logger failed")
	}
	ctrl.logger = lg
	// global
	global, err := newGlobal(ctrl.logger, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "initialize global failed")
	}
	ctrl.global = global
	// load proxy clients from database
	pcs, err := ctrl.db.SelectProxyClient()
	if err != nil {
		return nil, errors.Wrap(err, "load proxy clients failed")
	}
	for i := 0; i < len(pcs); i++ {
		tag := pcs[i].Tag
		client := &proxy.Client{
			Mode:   pcs[i].Mode,
			Config: pcs[i].Config,
		}
		err = ctrl.global.AddProxyClient(tag, client)
		if err != nil {
			return nil, errors.Wrapf(err, "add proxy client %s failed", tag)
		}
	}
	// load dns servers from database
	dss, err := ctrl.db.SelectDNSServer()
	if err != nil {
		return nil, errors.Wrap(err, "load dns servers failed")
	}
	for i := 0; i < len(dss); i++ {
		tag := dss[i].Tag
		server := &dns.Server{
			Method:  dss[i].Method,
			Address: dss[i].Address,
		}
		err = ctrl.global.AddDNSSever(tag, server)
		if err != nil {
			return nil, errors.Wrapf(err, "add dns server %s failed", tag)
		}
	}
	// load time syncer configs from database
	tcs, err := ctrl.db.SelectTimeSyncer()
	if err != nil {
		return nil, errors.Wrap(err, "select time syncer failed")
	}
	for i := 0; i < len(tcs); i++ {
		cfg := &timesync.Config{}
		err = toml.Unmarshal([]byte(tcs[i].Config), cfg)
		if err != nil {
			return nil, errors.Wrapf(err, "load time syncer config: %s failed", tcs[i].Tag)
		}
		tag := tcs[i].Tag
		err = ctrl.global.AddTimeSyncerConfig(tag, cfg)
		if err != nil {
			return nil, errors.Wrapf(err, "add time syncer config %s failed", tag)
		}
	}
	// handler
	ctrl.handler = &handler{ctx: ctrl}
	// sender
	sender, err := newSender(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "initialize sender failed")
	}
	ctrl.sender = sender
	// syncer
	syncer, err := newSyncer(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "initialize syncer failed")
	}
	ctrl.syncer = syncer
	// boot
	ctrl.boot = newBoot(ctrl)
	// http server
	web, err := newWeb(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "initialize web server failed")
	}
	ctrl.web = web
	// wait and exit
	ctrl.wait = make(chan struct{}, 2)
	ctrl.exit = make(chan error, 1)
	return ctrl, nil
}

func (ctrl *CTRL) Main() error {
	defer func() { ctrl.wait <- struct{}{} }()
	// first synchronize time
	if !ctrl.Debug.SkipTimeSyncer {
		err := ctrl.global.StartTimeSyncer()
		if err != nil {
			return ctrl.fatal(err, "synchronize time failed")
		}
	}
	now := ctrl.global.Now().Format(logger.TimeLayout)
	ctrl.logger.Println(logger.Info, "init", "time:", now)
	// start web server
	err := ctrl.web.Deploy()
	if err != nil {
		return ctrl.fatal(err, "deploy web server failed")
	}
	ctrl.logger.Println(logger.Info, "init", "http server:", ctrl.web.Address())
	ctrl.logger.Print(logger.Info, "init", "controller is running")
	go func() {
		// wait to load controller keys
		ctrl.global.WaitLoadKeys()
		ctrl.logger.Print(logger.Info, "init", "load keys successfully")
		// load boots
		ctrl.logger.Print(logger.Info, "init", "start discover bootstrap nodes")
		boots, err := ctrl.db.SelectBoot()
		if err != nil {
			ctrl.logger.Println(logger.Error, "init", "select boot failed:", err)
			return
		}
		for i := 0; i < len(boots); i++ {
			_ = ctrl.boot.Add(boots[i])
		}
	}()
	ctrl.wait <- struct{}{}
	return <-ctrl.exit
}

func (ctrl *CTRL) Exit(err error) {
	ctrl.once.Do(func() {
		ctrl.web.Close()
		ctrl.logger.Print(logger.Info, "exit", "web server is stopped")
		ctrl.boot.Close()
		ctrl.logger.Print(logger.Info, "exit", "boot is stopped")
		ctrl.syncer.Close()
		ctrl.logger.Print(logger.Info, "exit", "syncer is stopped")
		ctrl.sender.Close()
		ctrl.logger.Print(logger.Info, "exit", "sender is stopped")
		ctrl.global.Close()
		ctrl.logger.Print(logger.Info, "exit", "global is stopped")
		ctrl.logger.Print(logger.Info, "exit", "controller is stopped")
		ctrl.db.Close()
		// clean point
		ctrl.db = nil
		ctrl.logger = nil
		ctrl.global = nil
		ctrl.handler = nil
		ctrl.sender = nil
		ctrl.syncer = nil
		ctrl.boot = nil
		ctrl.web = nil
		ctrl.exit <- err
		close(ctrl.exit)
	})
}

func (ctrl *CTRL) fatal(err error, msg string) error {
	err = errors.WithMessage(err, msg)
	ctrl.logger.Println(logger.Fatal, "init", err)
	ctrl.Exit(nil)
	return err
}

func (ctrl *CTRL) LoadKeys(password string) error {
	return ctrl.global.LoadKeys(password)
}

func (ctrl *CTRL) DeleteNode(guid []byte) error {
	err := ctrl.db.DeleteNode(guid)
	if err != nil {
		return errors.Wrapf(err, "delete node %X failed", guid)
	}
	return nil
}

func (ctrl *CTRL) DeleteBeacon(guid []byte) error {
	err := ctrl.db.DeleteBeacon(guid)
	if err != nil {
		return errors.Wrapf(err, "delete beacon %X failed", guid)
	}
	return nil
}

func (ctrl *CTRL) DeleteNodeUnscoped(guid []byte) error {
	err := ctrl.db.DeleteNodeUnscoped(guid)
	if err != nil {
		return errors.Wrapf(err, "unscoped delete node %X failed", guid)
	}
	return nil
}

func (ctrl *CTRL) DeleteBeaconUnscoped(guid []byte) error {
	err := ctrl.db.DeleteBeaconUnscoped(guid)
	if err != nil {
		return errors.Wrapf(err, "unscoped delete beacon %X failed", guid)
	}
	return nil
}

// ------------------------------------test-------------------------------------

// TestWait is used to wait for Main()
func (ctrl *CTRL) TestWait() {
	<-ctrl.wait
}
