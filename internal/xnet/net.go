package xnet

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"project/internal/options"
	"project/internal/xnet/internal"
	"project/internal/xnet/light"
	"project/internal/xnet/quic"
	"project/internal/xnet/xtls"
)

type Mode = string

const (
	TLS   Mode = "tls"
	QUIC  Mode = "quic"
	HTTP  Mode = "http"
	HTTPS Mode = "https"
	Light Mode = "light"
)

var (
	ErrEmptyPort    = errors.New("empty port")
	ErrEmptyMode    = errors.New("empty mode")
	ErrEmptyNetwork = errors.New("empty network")
)

type InvalidPortError int

func (p InvalidPortError) Error() string {
	return fmt.Sprintf("invalid port: %d", p)
}

type UnknownModeError string

func (m UnknownModeError) Error() string {
	return fmt.Sprintf("unknown mode: %s", string(m))
}

type mismatchedModeNetwork struct {
	mode    string
	network string
}

func (mn *mismatchedModeNetwork) Error() string {
	return fmt.Sprintf("mismatched mode and network: %s %s",
		mn.mode, mn.network)
}

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
	return CheckPort(n)
}

func CheckPort(port int) error {
	if port < 1 || port > 65535 {
		return InvalidPortError(port)
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
			return &mismatchedModeNetwork{mode: mode, network: network}
		}
	case QUIC:
		switch network {
		case "udp", "udp4", "udp6":
		default:
			return &mismatchedModeNetwork{mode: mode, network: network}
		}
	case Light:
		switch network {
		case "tcp", "tcp4", "tcp6":
		default:
			return &mismatchedModeNetwork{mode: mode, network: network}
		}
	default:
		return UnknownModeError(mode)
	}
	return nil
}

func Listen(mode Mode, cfg *Config) (net.Listener, error) {
	switch mode {
	case TLS:
		err := CheckModeNetwork(TLS, cfg.Network)
		if err != nil {
			return nil, err
		}
		tlsConfig, err := cfg.TLSConfig.Apply()
		if err != nil {
			return nil, err
		}
		return xtls.Listen(cfg.Network, cfg.Address, tlsConfig, cfg.Timeout)
	case QUIC:
		err := CheckModeNetwork(QUIC, cfg.Network)
		if err != nil {
			return nil, err
		}
		tlsConfig, err := cfg.TLSConfig.Apply()
		if err != nil {
			return nil, err
		}
		return quic.Listen(cfg.Network, cfg.Address, tlsConfig, cfg.Timeout)
	case Light:
		err := CheckModeNetwork(Light, cfg.Network)
		if err != nil {
			return nil, err
		}
		return light.Listen(cfg.Network, cfg.Address, cfg.Timeout)
	default:
		return nil, UnknownModeError(mode)
	}
}

func Dial(mode Mode, cfg *Config) (net.Conn, error) {
	switch mode {
	case TLS:
		err := CheckModeNetwork(TLS, cfg.Network)
		if err != nil {
			return nil, err
		}
		tlsConfig, err := cfg.TLSConfig.Apply()
		if err != nil {
			return nil, err
		}
		return xtls.Dial(cfg.Network, cfg.Address, tlsConfig, cfg.Timeout)
	case QUIC:
		err := CheckModeNetwork(QUIC, cfg.Network)
		if err != nil {
			return nil, err
		}
		tlsConfig, err := cfg.TLSConfig.Apply()
		if err != nil {
			return nil, err
		}
		return quic.Dial(cfg.Network, cfg.Address, tlsConfig, cfg.Timeout)
	case Light:
		err := CheckModeNetwork(Light, cfg.Network)
		if err != nil {
			return nil, err
		}
		return light.Dial(cfg.Network, cfg.Address, cfg.Timeout)
	default:
		return nil, UnknownModeError(mode)
	}
}

// NewDeadlineConn return a net.Conn that
// set deadline before each Read() and Write()
func NewDeadlineConn(conn net.Conn, deadline time.Duration) net.Conn {
	return internal.NewDeadlineConn(conn, deadline)
}
