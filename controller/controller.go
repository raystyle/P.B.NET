package controller

import (
	"context"
	"io/ioutil"
	"sync"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/crypto/cert"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
)

// CTRL is controller
// broadcast messages to Nodes, send messages to Nodes or Beacons
type CTRL struct {
	Test *Test

	database  *database  // database
	logger    *gLogger   // global logger
	global    *global    // certificate, proxy, dns, time syncer, and ...
	syncer    *syncer    // receive message
	clientMgr *clientMgr // clients manager
	sender    *sender    // broadcast and send message
	handler   *handler   // handle message from Node or Beacon
	worker    *worker    // do work
	boot      *boot      // auto discover bootstrap node listeners
	web       *web       // web server

	once sync.Once
	wait chan struct{}
	exit chan error
}

// New is used to create controller from config
func New(cfg *Config) (*CTRL, error) {
	// copy test
	test := cfg.Test
	ctrl := &CTRL{Test: &test}
	// database
	database, err := newDatabase(cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize database")
	}
	ctrl.database = database
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
	// syncer
	syncer, err := newSyncer(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize syncer")
	}
	ctrl.syncer = syncer
	// client manager
	clientMgr, err := newClientManager(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize client manager")
	}
	ctrl.clientMgr = clientMgr
	// sender
	sender, err := newSender(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize sender")
	}
	ctrl.sender = sender
	// handler
	ctrl.handler = newHandler(ctrl)
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
	if !ctrl.Test.SkipTestClientDNS {
		err := ctrl.global.TestDNSOption(ctrl.clientMgr.GetDNSOptions())
		if err != nil {
			return errors.WithMessage(err, "failed to test client DNS option")
		}
	}
	// synchronize time
	if ctrl.Test.SkipSynchronizeTime {
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
	ctrl.logger.Print(logger.Info, "main", "start discover bootstrap node listeners")
	boots, err := ctrl.database.SelectBoot()
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
		ctrl.handler.Cancel()
		ctrl.worker.Close()
		ctrl.logger.Print(logger.Info, "exit", "worker is stopped")
		ctrl.handler.Close()
		ctrl.logger.Print(logger.Info, "exit", "handler is stopped")
		ctrl.sender.Close()
		ctrl.logger.Print(logger.Info, "exit", "sender is stopped")
		ctrl.clientMgr.Close()
		ctrl.logger.Print(logger.Info, "exit", "client manager is closed")
		ctrl.syncer.Close()
		ctrl.logger.Print(logger.Info, "exit", "syncer is stopped")
		ctrl.global.Close()
		ctrl.logger.Print(logger.Info, "exit", "global is stopped")
		ctrl.logger.Print(logger.Info, "exit", "controller is stopped")
		ctrl.logger.Close()
		ctrl.database.Close()
		ctrl.exit <- err
		close(ctrl.exit)
	})
}

// LoadSessionKey is used to load session key
func (ctrl *CTRL) LoadSessionKey(data, password []byte) error {
	return ctrl.global.LoadSessionKey(data, password)
}

// Synchronize is used to connect a node and start synchronize
func (ctrl *CTRL) Synchronize(ctx context.Context, guid *guid.GUID, bl *bootstrap.Listener) error {
	return ctrl.sender.Synchronize(ctx, guid, bl)
}

// Disconnect is used to disconnect node, guid is hex, upper
func (ctrl *CTRL) Disconnect(guid *guid.GUID) error {
	return ctrl.sender.Disconnect(guid)
}

// Send is used to send messages to Node or Beacon
func (ctrl *CTRL) Send(role protocol.Role, guid *guid.GUID, cmd []byte, msg interface{}) error {
	return ctrl.sender.Send(role, guid, cmd, msg)
}

// EnableInteractiveMode is used to enable Beacon interactive mode.
func (ctrl *CTRL) EnableInteractiveMode(guid *guid.GUID) {
	ctrl.sender.EnableInteractiveMode(guid)
}

// Broadcast is used to broadcast messages to all Nodes
func (ctrl *CTRL) Broadcast(cmd []byte, msg interface{}) error {
	return ctrl.sender.Broadcast(cmd, msg)
}

// DeleteNode is used to delete node
func (ctrl *CTRL) DeleteNode(guid *guid.GUID) error {
	err := ctrl.database.DeleteNode(guid)
	return errors.Wrapf(err, "failed to delete node %X", guid)
}

// DeleteNodeUnscoped is used to unscoped delete node
func (ctrl *CTRL) DeleteNodeUnscoped(guid *guid.GUID) error {
	err := ctrl.database.DeleteNodeUnscoped(guid)
	return errors.Wrapf(err, "failed to unscoped delete node %X", guid)
}

// DeleteBeacon is used to delete beacon
func (ctrl *CTRL) DeleteBeacon(guid *guid.GUID) error {
	err := ctrl.database.DeleteBeacon(guid)
	return errors.Wrapf(err, "failed to delete beacon %X", guid)
}

// DeleteBeaconUnscoped is used to unscoped delete beacon
func (ctrl *CTRL) DeleteBeaconUnscoped(guid *guid.GUID) error {
	err := ctrl.database.DeleteBeaconUnscoped(guid)
	return errors.Wrapf(err, "failed to unscoped delete beacon %X", guid)
}

// ------------------------------------test-------------------------------------

// LoadSessionKeyFromFile is used to load session key from file
func (ctrl *CTRL) LoadSessionKeyFromFile(filename string, password []byte) error {
	data, err := ioutil.ReadFile(filename) // #nosec
	if err != nil {
		return err
	}
	return ctrl.global.LoadSessionKey(data, password)
}

// KeyExchangePublicKey is used to get key exchange public key
func (ctrl *CTRL) KeyExchangePublicKey() []byte {
	return ctrl.global.KeyExchangePublicKey()
}

// PublicKey is used to get public key
func (ctrl *CTRL) PublicKey() []byte {
	return ctrl.global.PublicKey()
}

// BroadcastKey is used to get broadcast key
func (ctrl *CTRL) BroadcastKey() []byte {
	return ctrl.global.BroadcastKey()
}

// GetSelfCerts is used to get self certificates to generate CA-sign certificate
func (ctrl *CTRL) GetSelfCerts() []*cert.Pair {
	return ctrl.global.GetSelfCerts()
}

// GetSystemCerts is used to get system certificates
func (ctrl *CTRL) GetSystemCerts() []*cert.Pair {
	return ctrl.global.GetSystemCerts()
}
