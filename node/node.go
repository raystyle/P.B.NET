package node

import (
	"sync"

	"github.com/pkg/errors"

	"project/internal/config"
	"project/internal/logger"
)

type NODE struct {
	debug  *Debug
	logLv  logger.Level
	cache  *cache
	db     *db
	global *global
	syncer *syncer
	sender *sender
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
		cache: newCache(),
	}
	// init database
	db, err := newDB(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "init database failed")
	}
	node.db = db
	// init global
	global, err := newGlobal(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "init global failed")
	}
	node.global = global
	// init syncer
	syncer, err := newSyncer(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "init syncer failed")
	}
	node.syncer = syncer
	// init sender
	sender, err := newSender(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "init sender failed")
	}
	node.sender = sender
	// init server
	server, err := newServer(node, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "init server failed")
	}
	node.server = server
	// register
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
	node.Println(logger.Info, "init", "time:", now)
	// deploy server
	err := node.server.Deploy()
	if err != nil {
		return node.fatal(err, "deploy server failed")
	}
	node.Print(logger.Info, "init", "node is running")
	node.wait <- struct{}{}
	return <-node.exit
}

func (node *NODE) fatal(err error, msg string) error {
	err = errors.WithMessage(err, msg)
	node.Println(logger.Fatal, "init", err)
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
		node.Print(logger.Info, "exit", "web server is stopped")
		node.global.Destroy()
		node.Print(logger.Info, "exit", "global is stopped")
		node.Print(logger.Info, "exit", "node is stopped")
		node.exit <- err
		close(node.exit)
	})
}

func (node *NODE) AddListener(l *config.Listener) error {
	return node.server.AddListener(l)
}
