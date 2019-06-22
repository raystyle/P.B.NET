package xnet

import (
	"net"
	"time"

	"github.com/pelletier/go-toml"

	"project/internal/options"
	"project/internal/xnet/light"
	"project/internal/xnet/xtls"
)

type Config struct {
	Mode   Mode
	Config []byte // toml
}

func Listen(c *Config) (net.Listener, error) {
	switch c.Mode {
	case TLS:
		conf := &struct {
			Network    string             `toml:"network"`
			Address    string             `toml:"address"`
			TLS_Config options.TLS_Config `toml:"tls_config"`
		}{}
		err := toml.Unmarshal(c.Config, conf)
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
		return xtls.Listen(conf.Network, conf.Address, tls_config)
	case LIGHT:
		conf := &struct {
			Network string        `toml:"network"`
			Address string        `toml:"address"`
			Timeout time.Duration `toml:"timeout"`
		}{}
		err := toml.Unmarshal(c.Config, conf)
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
