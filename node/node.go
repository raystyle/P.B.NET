package node

import (
	"project/internal/logger"
)

type NODE struct {
	config *Config
	logger logger.Logger
	global *global
}

func New(c *Config) (*NODE, error) {
	node := &NODE{config: c}
	node.logger = new_log(node)
	global, err := new_global(c)
	if err != nil {
		return nil, err
	}
	node.global = global
	return node, nil
}

func (this *NODE) Main() error {
	return nil
}

func (this *NODE) Exit() error {
	return nil
}
