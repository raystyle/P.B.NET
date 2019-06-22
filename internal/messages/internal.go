package messages

import (
	"time"

	"project/internal/bootstrap"
	"project/internal/xnet"
)

type Bootstrap struct {
	Mode   bootstrap.Mode
	Config []byte
}

type Listener struct {
	Mode    xnet.Mode
	Config  []byte
	Timeout time.Duration // start timeout
}
