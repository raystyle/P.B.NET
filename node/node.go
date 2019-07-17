package node

import (
	"sync"

	"github.com/pkg/errors"

	"project/internal/logger"
)

type NODE struct {
	debug  *Debug
	log_lv logger.Level
	global *global
	server *server
	once   sync.Once
	wait   chan struct{}
	exit   chan error
}

func New(c *Config) (*NODE, error) {
	// init logger
	l, err := logger.Parse(c.Log_Level)
	if err != nil {
		return nil, err
	}
	// copy debug config
	debug := c.Debug
	node := &NODE{
		debug:  &debug,
		log_lv: l,
	}
	// init global
	g, err := new_global(node, c)
	if err != nil {
		return nil, err
	}
	node.global = g
	// init server
	s, err := new_server(node, c)
	if err != nil {
		return nil, err
	}
	node.server = s
	// init server
	if !c.Is_Genesis {
		err = node.register(c)
		if err != nil {
			return nil, err
		}
	}
	node.wait = make(chan struct{}, 2)
	node.exit = make(chan error, 1)
	return node, nil
}

func (this *NODE) Main() error {
	defer func() { this.wait <- struct{}{} }()
	// first synchronize time
	if !this.debug.Skip_Timesync {
		err := this.global.Start_Timesync()
		if err != nil {
			return this.fatal(err, "synchronize time failed")
		}
	}
	now := this.global.Now().Format(logger.Time_Layout)
	this.Println(logger.INFO, "init", "time:", now)
	err := this.server.Deploy()
	if err != nil {
		return this.fatal(err, "deploy server failed")
	}
	this.Print(logger.INFO, "init", "node is running")
	this.wait <- struct{}{}
	return <-this.exit
}

func (this *NODE) fatal(err error, msg string) error {
	err = errors.WithMessage(err, msg)
	this.Println(logger.FATAL, "init", err)
	this.Exit(nil)
	return err
}

// for Test wait for Main()
func (this *NODE) Wait() {
	<-this.wait
}

func (this *NODE) Exit(err error) {
	this.once.Do(func() {
		this.server.Shutdown()
		this.Print(logger.INFO, "exit", "web server is stopped")
		this.global.Close()
		this.Print(logger.INFO, "exit", "global is stopped")
		this.exit <- err
		close(this.exit)
		this.Print(logger.INFO, "exit", "node is stopped")
	})
}
