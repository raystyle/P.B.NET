package xnet

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"project/internal/xnet/light"
	"project/internal/xnet/quic"
	"project/internal/xnet/xtls"
)

var (
	ErrEmptyMode    = fmt.Errorf("empty mode")
	ErrEmptyNetwork = fmt.Errorf("empty network")
)

type Mode = string

const (
	TLS   Mode = "tls"
	QUIC  Mode = "quic"
	HTTP  Mode = "http"
	Light Mode = "light"
)

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

func CheckModeNetwork(mode string, network string) error {
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

type Dialer interface {
	Dial(network, address string) (net.Conn, error)
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
}

type Config struct {
	Network   string
	Address   string
	Timeout   time.Duration
	TLSConfig *tls.Config
	Dialer    Dialer
}

func Listen(mode Mode, cfg *Config) (net.Listener, error) {
	switch mode {
	case TLS:
		err := CheckModeNetwork(TLS, cfg.Network)
		if err != nil {
			return nil, err
		}
		return xtls.Listen(cfg.Network, cfg.Address, cfg.TLSConfig, cfg.Timeout)
	case QUIC:
		err := CheckModeNetwork(QUIC, cfg.Network)
		if err != nil {
			return nil, err
		}
		return quic.Listen(cfg.Network, cfg.Address, cfg.TLSConfig, cfg.Timeout)
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
		return xtls.Dial(
			cfg.Network,
			cfg.Address,
			cfg.TLSConfig,
			cfg.Timeout,
			cfg.Dialer.DialTimeout)
	case QUIC:
		err := CheckModeNetwork(QUIC, cfg.Network)
		if err != nil {
			return nil, err
		}
		return quic.Dial(
			cfg.Network,
			cfg.Address,
			cfg.TLSConfig,
			cfg.Timeout)
	case Light:
		err := CheckModeNetwork(Light, cfg.Network)
		if err != nil {
			return nil, err
		}
		return light.Dial(
			cfg.Network,
			cfg.Address,
			cfg.Timeout,
			cfg.Dialer.DialTimeout)
	default:
		return nil, UnknownModeError(mode)
	}
}
