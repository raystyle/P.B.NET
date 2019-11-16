package node

import (
	"sync"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/messages"
)

type NODE struct {
	Debug *Debug // for test

	logger  *gLogger // global logger
	global  *global  // proxy clients, DNS Clients, time syncer
	handler *handler // handle message from Controller
	sender  *sender  // send message to Controller
	syncer  *syncer  // receive message from Controller
	server  *server  // listen and serve Roles

	once sync.Once
	wait chan struct{}
	exit chan error
}

func New(cfg *Config) (*NODE, error) {
	// copy debug config
	debug := cfg.Debug
	node := &NODE{Debug: &debug}
	// logger
	lg, err := newLogger(node, cfg.LogLevel)
	if err != nil {
		return nil, errors.WithMessage(err, "initialize logger failed")
	}
	node.logger = lg
	// global
	global, err := newGlobal(node.logger, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "initialize global failed")
	}
	node.global = global
	// sender
	sender, err := newSender(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "initialize sender failed")
	}
	node.sender = sender
	// syncer
	syncer, err := newSyncer(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "initialize syncer failed")
	}
	node.syncer = syncer
	// server
	server, err := newServer(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "initialize server failed")
	}
	node.server = server
	node.wait = make(chan struct{}, 2)
	node.exit = make(chan error, 1)
	return node, nil
}

func (node *NODE) Main() error {
	defer func() { node.wait <- struct{}{} }()
	// first synchronize time
	if !node.Debug.SkipTimeSyncer {
		err := node.global.StartTimeSyncer()
		if err != nil {
			return node.fatal(err, "synchronize time failed")
		}
	}
	now := node.global.Now().Format(logger.TimeLayout)
	node.logger.Println(logger.Debug, "init", "time:", now)
	// register

	// deploy server
	err := node.server.Deploy()
	if err != nil {
		return node.fatal(err, "deploy server failed")
	}

	node.logger.Print(logger.Debug, "init", "node is running")
	node.wait <- struct{}{}
	return <-node.exit
}

func (node *NODE) fatal(err error, msg string) error {
	err = errors.WithMessage(err, msg)
	node.logger.Println(logger.Fatal, "init", err)
	node.Exit(nil)
	return err
}

func (node *NODE) Exit(err error) {
	node.once.Do(func() {
		node.server.Close()
		node.logger.Print(logger.Debug, "exit", "web server is stopped")
		node.syncer.Close()
		node.logger.Print(logger.Debug, "exit", "syncer is stopped")
		node.sender.Close()
		node.logger.Print(logger.Debug, "exit", "sender is stopped")
		node.global.Close()
		node.logger.Print(logger.Debug, "exit", "global is stopped")
		node.logger.Print(logger.Debug, "exit", "node is stopped")
		// clean point
		node.logger = nil
		node.global = nil
		node.handler = nil
		node.sender = nil
		node.syncer = nil
		node.server = nil
		node.exit <- err
		close(node.exit)
	})
}

func (node *NODE) AddListener(listener *messages.Listener) error {
	return node.server.AddListener(listener)
}

// ------------------------------------test-------------------------------------

// TestWaitMain is used to wait for Main()
func (node *NODE) TestWait() {
	<-node.wait
}

func (node *NODE) TestGetGUID() []byte {
	return node.global.GUID()
}

func (node *NODE) TestSend(msg []byte) error {
	return node.sender.Send(messages.TestBytes, msg)
}
