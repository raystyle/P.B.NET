package node

import (
	"time"

	"project/internal/logger"
)

type NODE struct {
	config    *Config
	logger    logger.Logger
	global    *global
	presenter *presenter
	server    *server
}

func New(c *Config) (*NODE, error) {
	node := &NODE{config: c}
	l, err := new_logger(node)
	if err != nil {
		return nil, err
	}
	node.logger = l
	g, err := new_global(node)
	if err != nil {
		return nil, err
	}
	node.global = g
	return node, nil
}

func (this *NODE) Main() error {
	err := this.global.Start_Timesync()
	if err != nil {
		return err
	}
	p, err := new_presenter(this)
	if err != nil {
		return err
	}
	this.presenter = p
	go p.Start()
	select {}
	time.Sleep(5 * time.Second)
	_ = this.Exit()
	return nil
}

func (this *NODE) Exit() error {
	this.presenter.Shutdown()
	return nil
}
