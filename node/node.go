package node

import (
	"sync"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/messages"
)

// Node
type Node struct {
	Debug *Debug // for test

	logger    *gLogger   // global logger
	global    *global    // proxy clients, DNS clients, time syncer
	opts      *opts      // client options
	forwarder *forwarder // forward messages
	sender    *sender    // send message to Controller
	syncer    *syncer    // receive message from Controller, Nodes, and Beacons
	handler   *handler   // handle message from Controller
	worker    *worker    // do work
	server    *server    // listen and serve Roles

	once sync.Once
	wait chan struct{}
	exit chan error
}

// New is used to create a Node from configuration
func New(cfg *Config) (*Node, error) {
	// copy debug config
	debug := cfg.Debug
	node := &Node{Debug: &debug}
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
	// copy client options
	node.opts = &opts{
		ProxyTag: cfg.Client.ProxyTag,
		Timeout:  cfg.Client.Timeout,
		DNSOpts:  cfg.Client.DNSOpts,
	}
	// forwarder
	forwarder, err := newForwarder(cfg)
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
	// syncer
	syncer, err := newSyncer(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize syncer")
	}
	node.syncer = syncer
	// handler
	node.handler = &handler{ctx: node}
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
	node.wait = make(chan struct{}, 2)
	node.exit = make(chan error, 1)
	return node, nil
}

func (node *Node) fatal(err error, msg string) error {
	err = errors.WithMessage(err, msg)
	node.logger.Println(logger.Fatal, "main", err)
	node.Exit(nil)
	return err
}

// Main is used to run
func (node *Node) Main() error {
	defer func() { node.wait <- struct{}{} }()
	// first synchronize time
	if !node.Debug.SkipTimeSyncer {
		err := node.global.StartTimeSyncer()
		if err != nil {
			return node.fatal(err, "failed to synchronize time")
		}
	}
	now := node.global.Now().Format(logger.TimeLayout)
	node.logger.Println(logger.Debug, "main", "time:", now)
	// register

	// deploy server
	err := node.server.Deploy()
	if err != nil {
		return node.fatal(err, "failed to deploy server")
	}

	node.logger.Print(logger.Debug, "main", "node is running")
	node.wait <- struct{}{}
	return <-node.exit
}

// Wait is used to wait for Main()
func (node *Node) Wait() {
	<-node.wait
}

// Exit is used to exit with a error
func (node *Node) Exit(err error) {
	node.once.Do(func() {
		node.server.Close()
		node.logger.Print(logger.Debug, "exit", "server is stopped")
		node.worker.Close()
		node.logger.Print(logger.Debug, "exit", "worker is stopped")
		node.syncer.Close()
		node.logger.Print(logger.Debug, "exit", "syncer is stopped")
		node.sender.Close()
		node.logger.Print(logger.Debug, "exit", "sender is stopped")
		node.global.Close()
		node.logger.Print(logger.Debug, "exit", "global is stopped")
		node.logger.Print(logger.Debug, "exit", "node is stopped")
		node.logger.Close()
		node.exit <- err
		close(node.exit)
	})
}

// AddListener is used to add listener
func (node *Node) AddListener(listener *messages.Listener) error {
	return node.server.AddListener(listener)
}

// GetListener is used to get listener
func (node *Node) GetListener(tag string) (*Listener, error) {
	return node.server.GetListener(tag)
}

// ------------------------------------test-------------------------------------

func (node *Node) TestGetGUID() []byte {
	return node.global.GUID()
}

func (node *Node) TestSend(msg []byte) error {
	return node.sender.Send(messages.TestBytes, msg)
}
