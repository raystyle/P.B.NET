package xnet

import (
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/pelletier/go-toml"

	"project/internal/options"
	"project/internal/xnet/light"
	"project/internal/xnet/xtls"
)

type Mode = string

const (
	TLS   Mode = "tls"
	LIGHT Mode = "light"
)

var (
	ERR_EMPTY_PORT              = errors.New("empty port")
	ERR_INVALID_PORT            = errors.New("invalid port")
	ERR_EMPTY_MODE              = errors.New("empty mode")
	ERR_EMPTY_NETWORK           = errors.New("empty network")
	ERR_UNKNOWN_MODE            = errors.New("unknown mode")
	ERR_MISMATCHED_MODE_NETWORK = errors.New("mismatched mode and network")
)

func Check_Port_str(port string) error {
	if port == "" {
		return ERR_EMPTY_PORT
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return err
	}
	if n < 1 || n > 65535 {
		return ERR_INVALID_PORT
	}
	return nil
}

func Check_Port_int(port int) error {
	if port < 1 || port > 65535 {
		return ERR_INVALID_PORT
	}
	return nil
}

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

func Listen(m Mode, config []byte) (net.Listener, error) {
	switch m {
	case TLS:
		conf := &struct {
			Network    string             `toml:"network"`
			Address    string             `toml:"address"`
			Timeout    time.Duration      `toml:"timeout"`
			TLS_Config options.TLS_Config `toml:"tls_config"`
		}{}
		err := toml.Unmarshal(config, conf)
		if err != nil {
			return nil, err
		}
		err = Check_Mode_Network(TLS, conf.Network)
		if err != nil {
			return nil, err
		}
		tls_config, err := conf.TLS_Config.Apply()
		if err != nil {
			return nil, err
		}
		return xtls.Listen(conf.Network, conf.Address, tls_config, conf.Timeout)
	case LIGHT:
		conf := &struct {
			Network string        `toml:"network"`
			Address string        `toml:"address"`
			Timeout time.Duration `toml:"timeout"`
		}{}
		err := toml.Unmarshal(config, conf)
		if err != nil {
			return nil, err
		}
		err = Check_Mode_Network(TLS, conf.Network)
		if err != nil {
			return nil, err
		}
		return light.Listen(conf.Network, conf.Address, conf.Timeout)
	default:
		return nil, ERR_UNKNOWN_MODE
	}
}
