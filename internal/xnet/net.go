package xnet

import (
	"net"
	"time"

	"github.com/pelletier/go-toml"

	"project/internal/options"
	"project/internal/xnet/light"
	"project/internal/xnet/xtls"
)

type Conn_Info struct {
	Connect_Time   int64
	Local_Network  string
	Local_Address  string
	Remote_Network string
	Remote_Address string
	Send           int
	Receive        int
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
