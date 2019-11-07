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

func Listen(mode string, config *Config) (net.Listener, error) {
	switch mode {
	case ModeTLS:
		err := CheckModeNetwork(ModeTLS, config.Network)
		if err != nil {
			return nil, err
		}
		return xtls.Listen(config.Network, config.Address, config.TLSConfig, config.Timeout)
	case ModeQUIC:
		err := CheckModeNetwork(ModeQUIC, config.Network)
		if err != nil {
			return nil, err
		}
		return quic.Listen(config.Network, config.Address, config.TLSConfig, config.Timeout)
	case ModeLight:
		err := CheckModeNetwork(ModeLight, config.Network)
		if err != nil {
			return nil, err
		}
		return light.Listen(config.Network, config.Address, config.Timeout)
	default:
		return nil, UnknownModeError(mode)
	}
}

func Dial(mode string, config *Config) (net.Conn, error) {
	return DialContext(context.Background(), mode, config)
}

func DialContext(ctx context.Context, mode string, config *Config) (net.Conn, error) {
	switch mode {
	case ModeTLS:
		err := CheckModeNetwork(ModeTLS, config.Network)
		if err != nil {
			return nil, err
		}
		return xtls.DialContext(
			ctx,
			config.Network,
			config.Address,
			config.TLSConfig,
			config.Timeout,
			config.Dialer,
		)
	case ModeQUIC:
		err := CheckModeNetwork(ModeQUIC, config.Network)
		if err != nil {
			return nil, err
		}
		return quic.DialContext(
			ctx,
			config.Network,
			config.Address,
			config.TLSConfig,
			config.Timeout,
		)
	case ModeLight:
		err := CheckModeNetwork(ModeLight, config.Network)
		if err != nil {
			return nil, err
		}
		return light.DialContext(
			ctx,
			config.Network,
			config.Address,
			config.Timeout,
			config.Dialer,
		)
	default:
		return nil, UnknownModeError(mode)
	}
}
