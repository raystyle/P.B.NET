package xnet

import (
	"errors"
)

type Mode = string

const (
	TLS   Mode = "tls"
	LIGHT Mode = "light"
)

var (
	ERR_EMPTY_MODE              = errors.New("empty mode")
	ERR_EMPTY_NETWORK           = errors.New("empty network")
	ERR_UNKNOWN_MODE            = errors.New("unknown mode")
	ERR_MISMATCHED_MODE_NETWORK = errors.New("mismatched mode and network")
)

func Check_Mode_Network(mode Mode, network string) error {
	if mode == "" {
		return ERR_EMPTY_MODE
	}
	if network == "" {
		return ERR_EMPTY_NETWORK
	}
	switch mode {
	case TLS:
		switch network {
		case "tcp", "tcp4", "tcp6":
		default:
			return ERR_MISMATCHED_MODE_NETWORK
		}
	case LIGHT:
		switch network {
		case "tcp", "tcp4", "tcp6":
		default:
			return ERR_MISMATCHED_MODE_NETWORK
		}
	default:
		return ERR_UNKNOWN_MODE
	}
	return nil
}
