package node

import (
	"sync"

	"github.com/pkg/errors"

	"project/internal/logger"
)

type NODE struct {
	debug  *Debug
	logLv  logger.Level
	global *global
	server *server
	once   sync.Once
	wait   chan struct{}
	exit   chan error
}

func New(cfg *Config) (*NODE, error) {
	// init logger
	lv, err := logger.Parse(cfg.LogLevel)
	if err != nil {
		return nil, err
	}
	// copy debug config
	debug := cfg.Debug
	node := &NODE{
		debug: &debug,
		logLv: lv,
	}
	// init global
	global, err := newGlobal(node, cfg)
	if err != nil {
		return nil, err
	}
	node.global = global
	// init server
	Server, err := newServer(node, cfg)
	if err != nil {
		return nil, err
	}
	node.server = Server
	// init server
	if !cfg.IsGenesis {
		err = node.register(cfg)
		if err != nil {
			return nil, err
		}
	}
	node.wait = make(chan struct{}, 2)
	node.exit = make(chan error, 1)
	return node, nil
}

func (node *NODE) Main() error {
	defer func() { node.wait <- struct{}{} }()
	// first synchronize time
	if !node.debug.SkipTimeSyncer {
		err := node.global.StartTimeSyncer()
		if err != nil {
			return node.fatal(err, "synchronize time failed")
		}
	}
	now := node.global.Now().Format(logger.TimeLayout)
	node.Println(logger.INFO, "init", "time:", now)
	err := node.server.Deploy()
	if err != nil {
		return node.fatal(err, "deploy server failed")
	}
	node.Print(logger.INFO, "init", "node is running")
	node.wait <- struct{}{}
	return <-node.exit
}

func (node *NODE) fatal(err error, msg string) error {
	err = errors.WithMessage(err, msg)
	node.Println(logger.FATAL, "init", err)
	node.Exit(nil)
	return err
}

// for Test wait for Main()
func (node *NODE) Wait() {
	<-node.wait
}

func (node *NODE) Exit(err error) {
	node.once.Do(func() {
		node.server.Shutdown()
		node.Print(logger.INFO, "exit", "web server is stopped")
		node.global.Destroy()
		node.Print(logger.INFO, "exit", "global is stopped")
		node.Print(logger.INFO, "exit", "node is stopped")
		node.exit <- err
		close(node.exit)
	})
}
