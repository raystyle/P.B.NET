package controller

import (
	"sync"

	"github.com/pkg/errors"

	"project/internal/logger"
)

type CTRL struct {
	Debug *Debug // for test

	db      *db      // database
	logger  *gLogger // global logger
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
		return nil, errors.WithMessage(err, "initialize global failed")
	}
	ctrl.global = global
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
	// wait to load controller keys
	if !ctrl.global.WaitLoadKeys() {
		return nil
	}
	ctrl.logger.Print(logger.Info, "init", "load keys successfully")
	// load boots
	ctrl.logger.Print(logger.Info, "init", "start discover bootstrap nodes")
	boots, err := ctrl.db.SelectBoot()
	if err != nil {
		ctrl.logger.Println(logger.Error, "init", "select boot failed:", err)
		return nil
	}
	for i := 0; i < len(boots); i++ {
		err = ctrl.boot.Add(boots[i])
		if err != nil {
			ctrl.logger.Println(logger.Error, "init", "add boot failed:", err)
		}
	}
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

		// clean points
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

func (ctrl *CTRL) LoadSessionKey(password string) error {
	return ctrl.global.LoadSessionKey(password)
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

// TestWaitMain is used to wait for Main()
func (ctrl *CTRL) TestWaitMain() {
	<-ctrl.wait
}
