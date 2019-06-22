package node

import (
	"time"

	"project/internal/logger"
)

type NODE struct {
	config *Config
	logger logger.Logger
	global *global
	server *server
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
	// err := this.global.Start_Timesync()
	// if err != nil {
	// 	return err
	// }
	go this.switch_register()
	select {}
	time.Sleep(2 * time.Second)
	return nil
}

func (this *NODE) Exit() error {
	return nil
}
