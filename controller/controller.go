package controller

import (
	"sync"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/crypto/cert"
	"project/internal/logger"
)

// CTRL is Controller
type CTRL struct {
	Debug *Debug

	db      *db      // database
	logger  *gLogger // global logger
	global  *global  // proxy, dns, time syncer, and ...
	opts    *opts    // client options
	sender  *sender  // broadcast and send message
	syncer  *syncer  // receive message
	handler *handler // handle message from Node or Beacon
	worker  *worker  // do work
	boot    *boot    // auto discover bootstrap nodes
	web     *web     // web server

	once sync.Once
	wait chan struct{}
	exit chan error
}

// New is used to create controller from config
func New(cfg *Config) (*CTRL, error) {
	// database
	db, err := newDB(cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize database")
	}
	// copy debug config
	debug := cfg.Debug
	ctrl := &CTRL{
		Debug: &debug,
		db:    db,
	}
	// logger
	lg, err := newLogger(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize logger")
	}
	ctrl.logger = lg
	// global
	global, err := newGlobal(ctrl.logger, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize global")
	}
	ctrl.global = global
	// copy client options
	ctrl.opts = &opts{
		ProxyTag: cfg.Client.ProxyTag,
		Timeout:  cfg.Client.Timeout,
		DNSOpts:  cfg.Client.DNSOpts,
	}
	// sender
	sender, err := newSender(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize sender")
	}
	ctrl.sender = sender
	// syncer
	syncer, err := newSyncer(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize syncer")
	}
	ctrl.syncer = syncer
	// handler
	ctrl.handler = &handler{ctx: ctrl}
	// worker
	worker, err := newWorker(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize worker")
	}
	ctrl.worker = worker
	// boot
	ctrl.boot = newBoot(ctrl)
	// http server
	web, err := newWeb(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize web server")
	}
	ctrl.web = web
	// wait and exit
	ctrl.wait = make(chan struct{}, 2)
	ctrl.exit = make(chan error, 1)
	return ctrl, nil
}

func (ctrl *CTRL) fatal(err error, msg string) error {
	err = errors.WithMessage(err, msg)
	ctrl.logger.Println(logger.Fatal, "main", err)
	ctrl.Exit(nil)
	return err
}

// Main is used to tun Controller, it will block until exit or return error
func (ctrl *CTRL) Main() error {
	defer func() { ctrl.wait <- struct{}{} }()
	// test client DNS option
	if !ctrl.Debug.SkipTestClientDNS {
		err := ctrl.global.TestDNSOption(&ctrl.opts.DNSOpts)
		if err != nil {
			return errors.WithMessage(err, "failed to test client DNS option")
		}
	}
	// synchronize time
	if ctrl.Debug.SkipSynchronizeTime {
		ctrl.global.StartTimeSyncerAddLoop()
	} else {
		err := ctrl.global.StartTimeSyncer()
		if err != nil {
			return ctrl.fatal(err, "failed to synchronize time")
		}
	}
	now := ctrl.global.Now().Format(logger.TimeLayout)
	ctrl.logger.Println(logger.Info, "main", "time:", now)
	// start web server
	err := ctrl.web.Deploy()
	if err != nil {
		return ctrl.fatal(err, "failed to deploy web server")
	}
	ctrl.logger.Printf(logger.Info, "main", "web server: https://%s/", ctrl.web.Address())
	ctrl.logger.Print(logger.Info, "main", "controller is running")
	// wait to load controller keys
	if !ctrl.global.WaitLoadSessionKey() {
		return nil
	}
	ctrl.logger.Print(logger.Info, "main", "load session key successfully")
	// load boots
	ctrl.logger.Print(logger.Info, "main", "start discover bootstrap nodes")
	boots, err := ctrl.db.SelectBoot()
	if err != nil {
		ctrl.logger.Println(logger.Error, "main", "failed to select boot:", err)
		return nil
	}
	for i := 0; i < len(boots); i++ {
		err = ctrl.boot.Add(boots[i])
		if err != nil {
			ctrl.logger.Println(logger.Error, "main", "failed to add boot:", err)
		}
	}
	ctrl.wait <- struct{}{}
	return <-ctrl.exit
}

// Wait is used to wait for Main()
func (ctrl *CTRL) Wait() {
	<-ctrl.wait
}

// Exit is used to exit with a error
func (ctrl *CTRL) Exit(err error) {
	ctrl.once.Do(func() {
		ctrl.web.Close()
		ctrl.logger.Print(logger.Info, "exit", "web server is stopped")
		ctrl.boot.Close()
		ctrl.logger.Print(logger.Info, "exit", "boot is stopped")
		// close worker first
		ctrl.syncer.Close()
		ctrl.logger.Print(logger.Info, "exit", "syncer is stopped")
		ctrl.sender.Close()
		ctrl.logger.Print(logger.Info, "exit", "sender is stopped")
		ctrl.global.Close()
		ctrl.logger.Print(logger.Info, "exit", "global is stopped")
		ctrl.logger.Print(logger.Info, "exit", "controller is stopped")
		ctrl.logger.Close()
		ctrl.db.Close()
		ctrl.exit <- err
		close(ctrl.exit)
	})
}

// LoadSessionKey is used to load session key
func (ctrl *CTRL) LoadSessionKey(password []byte) error {
	return ctrl.global.LoadSessionKey(password)
}

// KeyExchangePub is used to get key exchange public key
func (ctrl *CTRL) KeyExchangePub() []byte {
	return ctrl.global.KeyExchangePub()
}

// PublicKey is used to get public key
func (ctrl *CTRL) PublicKey() []byte {
	return ctrl.global.PublicKey()
}

// BroadcastKey is used to get broadcast key
func (ctrl *CTRL) BroadcastKey() []byte {
	return ctrl.global.BroadcastKey()
}

// GetSelfCA is used to get self CA certificate to generate CA-sign certificate
func (ctrl *CTRL) GetSelfCA() []*cert.KeyPair {
	return ctrl.global.GetSelfCA()
}

// Connect is used to connect node
func (ctrl *CTRL) Connect(node *bootstrap.Node, guid []byte) error {
	return ctrl.sender.Connect(node, guid)
}

// Disconnect is used to disconnect node, guid is hex, upper
func (ctrl *CTRL) Disconnect(guid string) error {
	return ctrl.sender.Disconnect(guid)
}

// DeleteNode is used to delete node
func (ctrl *CTRL) DeleteNode(guid []byte) error {
	err := ctrl.db.DeleteNode(guid)
	return errors.Wrapf(err, "failed to delete node %X", guid)
}

// DeleteBeacon is used to delete beacon
func (ctrl *CTRL) DeleteBeacon(guid []byte) error {
	err := ctrl.db.DeleteBeacon(guid)
	return errors.Wrapf(err, "failed to delete beacon %X", guid)
}

// DeleteNodeUnscoped is used to unscoped delete node
func (ctrl *CTRL) DeleteNodeUnscoped(guid []byte) error {
	err := ctrl.db.DeleteNodeUnscoped(guid)
	return errors.Wrapf(err, "failed to unscoped delete node %X", guid)
}

// DeleteBeaconUnscoped is used to unscoped delete beacon
func (ctrl *CTRL) DeleteBeaconUnscoped(guid []byte) error {
	err := ctrl.db.DeleteBeaconUnscoped(guid)
	return errors.Wrapf(err, "failed to unscoped delete beacon %X", guid)
}

// ------------------------------------test-------------------------------------
