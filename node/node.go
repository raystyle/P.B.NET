package node

import (
	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/protocol"
)

const (
	Version = protocol.V1_0_0
)

type NODE struct {
	log_level logger.Level
	global    *global
	server    *server
	exit      chan struct{}
}

func New(c *Config) (*NODE, error) {
	// init logger
	l, err := logger.Parse(c.Log_Level)
	if err != nil {
		return nil, err
	}
	node := &NODE{log_level: l}
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
			return nil, errors.WithMessage(err, "create server failed")
		}
		node.server = s
	} else {
		err = node.register(c)
		if err != nil {
			return nil, err
		}
	}
	node.exit = make(chan struct{}, 1)
	return node, nil
}

func (this *NODE) Main() error {
	err := this.server.Deploy()
	if err != nil {
		err = errors.WithMessage(err, "deploy server failed")
		this.Fatalln(err)
		return err
	}
	<-this.exit
	return nil
}

func (this *NODE) Exit() {
	close(this.exit)
}
