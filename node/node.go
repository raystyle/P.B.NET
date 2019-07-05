package node

import (
	"sync"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/protocol"
)

const (
	Version = protocol.V1_0_0
)

type NODE struct {
	log_lv logger.Level
	global *global
	server *server
	wg     sync.WaitGroup
	once   sync.Once
	exit   chan error
}

func New(c *Config) (*NODE, error) {
	// init logger
	l, err := logger.Parse(c.Log_Level)
	if err != nil {
		return nil, err
	}
	node := &NODE{log_lv: l}
	// init global
	g, err := new_global(node, c)
	if err != nil {
		return nil, err
	}
	node.global = g
	// init server
	if c.Is_Genesis {
		s, err := new_server(node, c)
		if err != nil {
			return nil, err
		}
		node.server = s
	} else {
		err = node.register(c)
		if err != nil {
			return nil, err
		}
	}
	node.exit = make(chan error)
	return node, nil
}

func (this *NODE) Main() error {
	// first synchronize time
	err := this.global.Start_Timesync()
	if err != nil {
		return this.fatal(err, "synchronize time failed")
	}
	err = this.server.Deploy()
	if err != nil {
		return this.fatal(err, "deploy server failed")
	}
	return <-this.exit
}

func (this *NODE) fatal(err error, msg string) error {
	err = errors.WithMessage(err, msg)
	this.Println(logger.FATAL, "init", err)
	this.Exit(nil)
	return err
}
func (this *NODE) Exit(err error) {
	this.once.Do(func() {
		// TODO race
		if this.server != nil {
			this.server.Shutdown()
			this.exit_log("web server is stopped")
		}
		this.wg.Wait()
		this.global.Close()
		this.exit_log("global is stopped")
		this.exit_log("node is stopped")
		if this.exit != nil {
			this.exit <- err
			close(this.exit)
		}
	})
}

func (this *NODE) exit_log(log string) {
	this.Print(logger.INFO, "exit", log)
}
