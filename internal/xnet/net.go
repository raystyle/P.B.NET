package xnet

import (
	"context"
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

const (
	ModeTLS   = "tls"
	ModeQUIC  = "quic"
	ModeLight = "light"
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
	case ModeTLS:
		switch network {
		case "tcp", "tcp4", "tcp6":
		default:
			return &mismatchedModeNetwork{mode: mode, network: network}
		}
	case ModeQUIC:
		switch network {
		case "udp", "udp4", "udp6":
		default:
			return &mismatchedModeNetwork{mode: mode, network: network}
		}
	case ModeLight:
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

type Dialer func(ctx context.Context, network, address string) (net.Conn, error)

type Config struct {
	Network   string
	Address   string
	Timeout   time.Duration
	TLSConfig *tls.Config
	Dialer    Dialer
}

func Listen(mode string, cfg *Config) (net.Listener, error) {
	switch mode {
	case ModeTLS:
		err := CheckModeNetwork(ModeTLS, cfg.Network)
		if err != nil {
			return nil, err
		}
		return xtls.Listen(cfg.Network, cfg.Address, cfg.TLSConfig, cfg.Timeout)
	case ModeQUIC:
		err := CheckModeNetwork(ModeQUIC, cfg.Network)
		if err != nil {
			return nil, err
		}
		return quic.Listen(cfg.Network, cfg.Address, cfg.TLSConfig, cfg.Timeout)
	case ModeLight:
		err := CheckModeNetwork(ModeLight, cfg.Network)
		if err != nil {
			return nil, err
		}
		return light.Listen(cfg.Network, cfg.Address, cfg.Timeout)
	default:
		return nil, UnknownModeError(mode)
	}
}

func Dial(mode string, config *Config) (net.Conn, error) {
	return DialContext(context.Background(), mode, config)
}

func DialContext(ctx context.Context, mode string, cfg *Config) (net.Conn, error) {
	switch mode {
	case ModeTLS:
		err := CheckModeNetwork(ModeTLS, cfg.Network)
		if err != nil {
			return nil, err
		}
		return xtls.DialContext(
			ctx,
			cfg.Network,
			cfg.Address,
			cfg.TLSConfig,
			cfg.Timeout,
			cfg.Dialer,
		)
	case ModeQUIC:
		err := CheckModeNetwork(ModeQUIC, cfg.Network)
		if err != nil {
			return nil, err
		}
		return quic.DialContext(
			ctx,
			cfg.Network,
			cfg.Address,
			cfg.TLSConfig,
			cfg.Timeout,
		)
	case ModeLight:
		err := CheckModeNetwork(ModeLight, cfg.Network)
		if err != nil {
			return nil, err
		}
		return light.DialContext(
			ctx,
			cfg.Network,
			cfg.Address,
			cfg.Timeout,
			cfg.Dialer,
		)
	default:
		return nil, UnknownModeError(mode)
	}
}
