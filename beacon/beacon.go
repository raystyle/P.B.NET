package beacon

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/guid"
	"project/internal/logger"
)

// Beacon send messages to Controller.
type Beacon struct {
	Test *Test

	logger    *gLogger   // global logger
	global    *global    // certificate, proxy, dns, time syncer, and ...
	syncer    *syncer    // sync network guid
	clientMgr *clientMgr // clients manager
	register  *register  // about register to Controller
	sender    *sender    // send message to controller
	handler   *handler   // handle message from controller
	worker    *worker    // do work
	driver    *driver    // control all modules

	once sync.Once
	wait chan struct{}
	exit chan error
}

// New is used to create a Beacon from configuration.
func New(cfg *Config) (*Beacon, error) {
	// copy test
	test := new(Test)
	test.options = cfg.Test
	beacon := &Beacon{Test: test}
	// logger
	lg, err := newLogger(beacon, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize logger")
	}
	beacon.logger = lg
	// global
	global, err := newGlobal(beacon.logger, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize global")
	}
	beacon.global = global
	// syncer
	syncer, err := newSyncer(beacon, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize syncer")
	}
	beacon.syncer = syncer
	// client manager
	clientMgr, err := newClientManager(beacon, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize client manager")
	}
	beacon.clientMgr = clientMgr
	// register
	register, err := newRegister(beacon, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize register")
	}
	beacon.register = register
	// sender
	sender, err := newSender(beacon, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize sender")
	}
	beacon.sender = sender
	// handler
	beacon.handler = newHandler(beacon)
	// worker
	worker, err := newWorker(beacon, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize worker")
	}
	beacon.worker = worker
	// driver
	driver, err := newDriver(beacon, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize worker")
	}
	beacon.driver = driver
	beacon.wait = make(chan struct{})
	beacon.exit = make(chan error, 1)
	return beacon, nil
}

// HijackLogWriter is used to hijack all packages that use log.Print().
func (beacon *Beacon) HijackLogWriter() {
	logger.HijackLogWriter(beacon.logger)
}

func (beacon *Beacon) fatal(err error, msg string) error {
	err = errors.WithMessage(err, msg)
	beacon.logger.Println(logger.Fatal, "main", err)
	beacon.Exit(nil)
	close(beacon.wait)
	return err
}

// Main is used to run Beacon, it will block until exit or return error.
func (beacon *Beacon) Main() error {
	const src = "main"
	// start log sender
	beacon.logger.StartSender()
	// synchronize time
	if beacon.Test.options.SkipSynchronizeTime {
		beacon.global.StartTimeSyncerWalker()
	} else {
		err := beacon.global.StartTimeSyncer()
		if err != nil {
			return beacon.fatal(err, "failed to synchronize time")
		}
	}
	now := beacon.global.Now().Local().Format(logger.TimeLayout)
	beacon.logger.Println(logger.Info, src, "time:", now)
	// start register
	err := beacon.register.Register()
	if err != nil {
		return beacon.fatal(err, "failed to register")
	}
	beacon.driver.Drive()
	beacon.logger.Print(logger.Info, src, "running")
	close(beacon.wait)
	return <-beacon.exit
}

// Wait is used to wait for Main().
func (beacon *Beacon) Wait() {
	<-beacon.wait
}

// Exit is used to exit with a error.
func (beacon *Beacon) Exit(err error) {
	const src = "exit"
	beacon.once.Do(func() {
		beacon.logger.CloseSender()
		beacon.driver.Close()
		beacon.logger.Print(logger.Info, src, "driver is stopped")
		beacon.handler.Cancel()
		beacon.worker.Close()
		beacon.logger.Print(logger.Info, src, "worker is stopped")
		beacon.handler.Close()
		beacon.logger.Print(logger.Info, src, "handler is stopped")
		beacon.sender.Close()
		beacon.logger.Print(logger.Info, src, "sender is stopped")
		beacon.register.Close()
		beacon.logger.Print(logger.Info, src, "register is closed")
		beacon.clientMgr.Close()
		beacon.logger.Print(logger.Info, src, "client manager is closed")
		beacon.syncer.Close()
		beacon.logger.Print(logger.Info, src, "syncer is stopped")
		beacon.global.Close()
		beacon.logger.Print(logger.Info, src, "global is closed")
		beacon.logger.Print(logger.Info, src, "beacon is stopped")
		beacon.logger.Close()
		beacon.exit <- err
		close(beacon.exit)
	})
}

// GUID is used to get Beacon GUID.
func (beacon *Beacon) GUID() *guid.GUID {
	return beacon.global.GUID()
}

// Synchronize is used to connect a Node and start to synchronize.
func (beacon *Beacon) Synchronize(ctx context.Context, guid *guid.GUID, bl *bootstrap.Listener) error {
	return beacon.sender.Synchronize(ctx, guid, bl)
}

// Send is used to send message to Controller.
func (beacon *Beacon) Send(ctx context.Context, command, message []byte, deflate bool) error {
	return beacon.sender.Send(ctx, command, message, deflate)
}

// Query is used to query message from Controller.
func (beacon *Beacon) Query() error {
	return beacon.sender.Query()
}
