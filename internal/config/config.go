package config

import (
	"project/internal/bootstrap"
	"project/internal/xnet"
)

type Bootstrap struct {
	Tag    string
	Mode   bootstrap.Mode
	Config []byte
}

type Listener struct {
	Tag    string
	Mode   xnet.Mode
	Config []byte
}
