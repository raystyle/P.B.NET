package controller

import (
	"encoding/base64"
	"sync"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/dns"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/timesync"
)

const (
	Name = "P.B.NET"
)

// TODO point

type CTRL struct {
	Debug   *Debug
	logLv   logger.Level
	cache   *cache   // database cache
	db      *db      // provide data
	global  *global  // proxy, dns, time syncer, and ...
	handler *handler // handle message from Node or Beacon
	syncer  *syncer  // sync message
	sender  *sender  // broadcast and send message
	boot    *boot    // auto discover bootstrap nodes
	web     *web     // web server
	once    sync.Once
	wait    chan struct{}
	exit    chan error
}

func New(cfg *Config) (*CTRL, error) {
	// init logger
	logLevel, err := logger.Parse(cfg.LogLevel)
	if err != nil {
		return nil, err
	}
	// copy debug config
	debug := cfg.Debug
	ctrl := &CTRL{
		Debug: &debug,
		logLv: logLevel,
		cache: newCache(),
	}
	// init database
	db, err := newDB(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "init database failed")
	}
	ctrl.db = db
	// init global
	global, err := newGlobal(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "init global failed")
	}
	ctrl.global = global
	// handler
	ctrl.handler = &handler{ctx: ctrl}
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
	// init syncer
	syncer, err := newSyncer(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "init syncer failed")
	}
	ctrl.syncer = syncer
	// init sender
	sender, err := newSender(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "init sender failed")
	}
	ctrl.sender = sender
	// init boot
	ctrl.boot = newBoot(ctrl)
	// init http server
	web, err := newWeb(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "init web server failed")
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
	ctrl.Println(logger.Info, "init", "time:", now)
	// start web server
	err := ctrl.web.Deploy()
	if err != nil {
		return ctrl.fatal(err, "deploy web server failed")
	}
	ctrl.Println(logger.Info, "init", "http server:", ctrl.web.Address())
	ctrl.Print(logger.Info, "init", "controller is running")
	go func() {
		// wait to load controller keys
		ctrl.global.WaitLoadKeys()
		ctrl.Print(logger.Info, "init", "load keys successfully")
		// load boots
		ctrl.Print(logger.Info, "init", "start discover bootstrap nodes")
		boots, err := ctrl.db.SelectBoot()
		if err != nil {
			ctrl.Println(logger.Error, "init", "select boot failed:", err)
			return
		}
		for i := 0; i < len(boots); i++ {
			_ = ctrl.boot.Add(boots[i])
		}
	}()
	ctrl.wait <- struct{}{}
	return <-ctrl.exit
}

func (ctrl *CTRL) fatal(err error, msg string) error {
	err = errors.WithMessage(err, msg)
	ctrl.Println(logger.Fatal, "init", err)
	ctrl.Exit(nil)
	return err
}

func (ctrl *CTRL) Exit(err error) {
	ctrl.once.Do(func() {
		ctrl.web.Close()
		ctrl.Print(logger.Info, "exit", "web server is stopped")
		ctrl.boot.Close()
		ctrl.Print(logger.Info, "exit", "boot is stopped")
		ctrl.sender.Close()
		ctrl.Print(logger.Info, "exit", "sender is stopped")
		ctrl.syncer.Close()
		ctrl.Print(logger.Info, "exit", "syncer is stopped")
		ctrl.global.Destroy()
		ctrl.Print(logger.Info, "exit", "global is stopped")
		ctrl.Print(logger.Info, "exit", "controller is stopped")
		ctrl.db.Close()
		ctrl.exit <- err
		close(ctrl.exit)
	})
}

func (ctrl *CTRL) LoadKeys(password string) error {
	return ctrl.global.LoadKeys(password)
}

func (ctrl *CTRL) DeleteNode(guid []byte) error {
	err := ctrl.db.DeleteNode(guid)
	if err != nil {
		return errors.Wrapf(err, "delete node %X failed", guid)
	}
	ctrl.deleteNode(guid)
	return nil
}

func (ctrl *CTRL) DeleteNodeUnscoped(guid []byte) error {
	err := ctrl.db.DeleteNodeUnscoped(guid)
	if err != nil {
		return errors.Wrapf(err, "unscoped delete node %X failed", guid)
	}
	ctrl.deleteNode(guid)
	return nil
}

func (ctrl *CTRL) deleteNode(guid []byte) {
	guidStr := base64.StdEncoding.EncodeToString(guid)
	ctrl.cache.DeleteNode(guidStr)
}

func (ctrl *CTRL) DeleteBeacon(guid []byte) error {
	err := ctrl.db.DeleteBeacon(guid)
	if err != nil {
		return errors.Wrapf(err, "delete beacon %X failed", guid)
	}
	ctrl.deleteBeacon(guid)
	return nil
}

func (ctrl *CTRL) DeleteBeaconUnscoped(guid []byte) error {
	err := ctrl.db.DeleteBeaconUnscoped(guid)
	if err != nil {
		return errors.Wrapf(err, "unscoped delete beacon %X failed", guid)
	}
	ctrl.deleteBeacon(guid)
	return nil
}

func (ctrl *CTRL) deleteBeacon(guid []byte) {
	guidStr := base64.StdEncoding.EncodeToString(guid)
	ctrl.cache.DeleteBeacon(guidStr)
}

func (ctrl *CTRL) Connect(node *bootstrap.Node, guid []byte) error {
	return ctrl.syncer.Connect(node, guid)
}

// ------------------------------------test-------------------------------------

// TestWait is used to wait for Main()
func (ctrl *CTRL) TestWait() {
	<-ctrl.wait
}

// TestSyncDBCache is used to synchronize cache to database immediately
func (ctrl *CTRL) TestSyncDBCache() {
	ctrl.db.cacheSyncer.Sync()
}
