package node

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/xnet"
)

// Node send messages to controller.
type Node struct {
	Test *Test

	storage    *storage    // storage
	logger     *gLogger    // global logger
	global     *global     // certificate, proxy, dns, time syncer, and ...
	syncer     *syncer     // sync network guid
	clientMgr  *clientMgr  // clients manager
	register   *register   // about register to Controller
	forwarder  *forwarder  // forward messages
	sender     *sender     // send message to controller
	messageMgr *messageMgr // message manager
	handler    *handler    // handle message from controller
	worker     *worker     // do work
	server     *server     // listen and serve Roles
	driver     *driver     // control all modules

	once sync.Once
	wait chan struct{}
	exit chan error
}

// New is used to create a Node from configuration.
func New(cfg *Config) (*Node, error) {
	// copy test
	test := new(Test)
	test.options = cfg.Test
	node := &Node{Test: test}
	// storage
	node.storage = newStorage()
	// logger
	lg, err := newLogger(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize logger")
	}
	node.logger = lg
	// global
	global, err := newGlobal(node.logger, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize global")
	}
	node.global = global
	// syncer
	syncer, err := newSyncer(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize syncer")
	}
	node.syncer = syncer
	// client manager
	clientMgr, err := newClientManager(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize client manager")
	}
	node.clientMgr = clientMgr
	// register
	register, err := newRegister(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize register")
	}
	node.register = register
	// forwarder
	forwarder, err := newForwarder(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize forwarder")
	}
	node.forwarder = forwarder
	// sender
	sender, err := newSender(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize sender")
	}
	node.sender = sender
	// message manager
	node.messageMgr = newMessageManager(node, cfg)
	// handler
	node.handler = newHandler(node)
	// worker
	worker, err := newWorker(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize worker")
	}
	node.worker = worker
	// server
	server, err := newServer(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize server")
	}
	node.server = server
	// driver
	driver, err := newDriver(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize worker")
	}
	node.driver = driver
	node.wait = make(chan struct{})
	node.exit = make(chan error, 1)
	return node, nil
}

// HijackLogWriter is used to hijack all packages that use log.Print().
func (node *Node) HijackLogWriter() {
	logger.HijackLogWriter(node.logger)
}

func (node *Node) fatal(err error, msg string) error {
	err = errors.WithMessage(err, msg)
	node.logger.Println(logger.Fatal, "main", err)
	node.Exit(nil)
	close(node.wait)
	return err
}

// Main is used to run Node, it will block until exit or return error.
func (node *Node) Main() error {
	const src = "main"
	// start log sender
	node.logger.StartSender()
	// synchronize time
	if node.Test.options.SkipSynchronizeTime {
		node.global.StartTimeSyncerWalker()
	} else {
		err := node.global.StartTimeSyncer()
		if err != nil {
			return node.fatal(err, "failed to synchronize time")
		}
	}
	now := node.global.Now().Local()
	node.global.SetStartupTime(now)
	nowStr := now.Format(logger.TimeLayout)
	node.logger.Println(logger.Info, src, "time:", nowStr)
	// deploy server
	err := node.server.Deploy()
	if err != nil {
		return node.fatal(err, "failed to deploy server")
	}
	// start register
	err = node.register.Register()
	if err != nil {
		return node.fatal(err, "failed to register")
	}
	// driver
	node.driver.Drive()
	node.logger.Print(logger.Info, src, "running")
	close(node.wait)
	return <-node.exit
}

// Wait is used to wait for Main().
func (node *Node) Wait() {
	<-node.wait
}

// Exit is used to exit with a error.
func (node *Node) Exit(err error) {
	const src = "exit"
	node.once.Do(func() {
		node.logger.CloseSender()
		node.driver.Close()
		node.logger.Print(logger.Info, src, "driver is stopped")
		node.handler.Cancel()
		node.server.Close()
		node.logger.Print(logger.Info, src, "server is stopped")
		node.worker.Close()
		node.logger.Print(logger.Info, src, "worker is stopped")
		node.handler.Close()
		node.logger.Print(logger.Info, src, "handler is stopped")
		node.messageMgr.Close()
		node.logger.Print(logger.Info, src, "message manager is stopped")
		node.sender.Close()
		node.logger.Print(logger.Info, src, "sender is stopped")
		node.forwarder.Close()
		node.logger.Print(logger.Info, src, "forwarder is stopped")
		node.register.Close()
		node.logger.Print(logger.Info, src, "register is closed")
		node.clientMgr.Close()
		node.logger.Print(logger.Info, src, "client manager is closed")
		node.syncer.Close()
		node.logger.Print(logger.Info, src, "syncer is stopped")
		node.global.Close()
		node.logger.Print(logger.Info, src, "global is closed")
		node.logger.Print(logger.Info, src, "node is stopped")
		node.logger.Close()
		node.exit <- err
		close(node.exit)
	})
}

// GUID is used to get Node GUID.
func (node *Node) GUID() *guid.GUID {
	return node.global.GUID()
}

// Synchronize is used to connect a Node and start to synchronize.
func (node *Node) Synchronize(
	ctx context.Context,
	guid *guid.GUID,
	listener *bootstrap.Listener,
) error {
	return node.sender.Synchronize(ctx, guid, listener)
}

// Disconnect is used to disconnect Node.
func (node *Node) Disconnect(guid *guid.GUID) error {
	return node.sender.Disconnect(guid)
}

// Send is used to send message to Controller.
func (node *Node) Send(
	ctx context.Context,
	command []byte,
	message []byte,
	deflate bool,
) error {
	return node.sender.Send(ctx, command, message, deflate)
}

// SendRT is used to send message to Controller and get response.
func (node *Node) SendRT(
	ctx context.Context,
	command []byte,
	message messages.RoundTripper,
	deflate bool,
	timeout time.Duration,
) (interface{}, error) {
	return node.messageMgr.Send(ctx, command, message, deflate, timeout)
}

// AddListener is used to add listener.
func (node *Node) AddListener(listener *messages.Listener) error {
	return node.server.AddListener(listener)
}

// GetListener is used to get listener.
func (node *Node) GetListener(tag string) (*xnet.Listener, error) {
	return node.server.GetListener(tag)
}
