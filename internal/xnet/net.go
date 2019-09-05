package xnet

import (
	"errors"
	"net"
	"strconv"
	"time"

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
	ErrEmptyPort             = errors.New("empty port")
	ErrInvalidPort           = errors.New("invalid port")
	ErrEmptyMode             = errors.New("empty mode")
	ErrEmptyNetwork          = errors.New("empty network")
	ErrUnknownMode           = errors.New("unknown mode")
	ErrMismatchedModeNetwork = errors.New("mismatched mode and network")
)

type Config struct {
	Network   string            `toml:"network"`
	Address   string            `toml:"address"`
	Timeout   time.Duration     `toml:"timeout"`
	TLSConfig options.TLSConfig `toml:"tls_config"`
}

func CheckPortString(port string) error {
	if port == "" {
		return ErrEmptyPort
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return err
	}
	if n < 1 || n > 65535 {
		return ErrInvalidPort
	}
	return nil
}

func CheckPortInt(port int) error {
	if port < 1 || port > 65535 {
		return ErrInvalidPort
	}
	return nil
}

func CheckModeNetwork(mode Mode, network string) error {
	if mode == "" {
		return ErrEmptyMode
	}
	if network == "" {
		return ErrEmptyNetwork
	}
	switch mode {
	case TLS:
		switch network {
		case "tcp", "tcp4", "tcp6":
		default:
			return ErrMismatchedModeNetwork
		}
	case LIGHT:
		switch network {
		case "tcp", "tcp4", "tcp6":
		default:
			return ErrMismatchedModeNetwork
		}
	default:
		return ErrUnknownMode
	}
	return nil
}

func Listen(m Mode, c *Config) (net.Listener, error) {
	switch m {
	case TLS:
		err := CheckModeNetwork(TLS, c.Network)
		if err != nil {
			return nil, err
		}
		tlsConfig, err := c.TLSConfig.Apply()
		if err != nil {
			return nil, err
		}
		return xtls.Listen(c.Network, c.Address, tlsConfig, c.Timeout)
	case LIGHT:
		err := CheckModeNetwork(TLS, c.Network)
		if err != nil {
			return nil, err
		}
		return light.Listen(c.Network, c.Address, c.Timeout)
	default:
		return nil, ErrUnknownMode
	}
}

func Dial(m Mode, c *Config) (net.Conn, error) {
	switch m {
	case TLS:
		err := CheckModeNetwork(TLS, c.Network)
		if err != nil {
			return nil, err
		}
		tlsConfig, err := c.TLSConfig.Apply()
		if err != nil {
			return nil, err
		}
		return xtls.Dial(c.Network, c.Address, tlsConfig, c.Timeout)
	case LIGHT:
		err := CheckModeNetwork(TLS, c.Network)
		if err != nil {
			return nil, err
		}
		return light.Dial(c.Network, c.Address, c.Timeout)
	default:
		return nil, ErrUnknownMode
	}
}
