package timesync

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/dns"
	"project/internal/proxy"
	"project/internal/timesync/ntp"
)

var ErrQueryNTPFailed = errors.New("query ntp server failed")

type NTPClient struct {
	proxyPool *proxy.Pool
	dnsClient *dns.Client

	Network  string        `toml:"network"`
	Address  string        `toml:"address"`
	Timeout  time.Duration `toml:"timeout"`
	Version  int           `toml:"version"`
	ProxyTag string        `toml:"proxy_tag"`
	DNSOpts  dns.Options   `toml:"dns_options"`
}

func NewNTPClient(config []byte) (*NTPClient, error) {
	nc := NTPClient{}
	err := toml.Unmarshal(config, &nc)
	if err != nil {
		return nil, err
	}
	return &nc, nil
}

func (client *NTPClient) Query() (now time.Time, isOptsErr bool, err error) {
	// check network
	switch client.Network {
	case "", "udp", "udp4", "udp6":
	default:
		isOptsErr = true
		err = fmt.Errorf("unknown network: %s", client.Network)
		return
	}
	host, port, err := net.SplitHostPort(client.Address)
	if err != nil {
		isOptsErr = true
		return
	}
	ntpOpts := ntp.Options{
		Network: client.Network,
		Timeout: client.Timeout,
		Version: client.Version,
	}
	// set proxy
	p, err := client.proxyPool.Get(client.ProxyTag)
	if err != nil {
		isOptsErr = true
		return
	}
	ntpOpts.Dial = p.DialTimeout
	// dns
	ipList, err := client.dnsClient.Resolve(host, &client.DNSOpts)
	if err != nil {
		isOptsErr = true
		err = fmt.Errorf("resolve domain name failed: %s", err)
		return
	}
	var resp *ntp.Response
	switch client.DNSOpts.Type {
	case "", dns.IPv4:
		for i := 0; i < len(ipList); i++ {
			resp, err = ntp.Query(ipList[i]+":"+port, &ntpOpts)
			if err == nil {
				now = resp.Time
				return
			}
		}
	case dns.IPv6:
		for i := 0; i < len(ipList); i++ {
			resp, err = ntp.Query("["+ipList[i]+"]:"+port, &ntpOpts)
			if err == nil {
				now = resp.Time
				return
			}
		}
	default:
		err = fmt.Errorf("timesyncer internal error: %s",
			dns.UnknownTypeError(client.DNSOpts.Type))
		panic(err)
	}
	err = ErrQueryNTPFailed
	return
}

func (client *NTPClient) ImportConfig(b []byte) error {
	return msgpack.Unmarshal(b, client)
}

func (client *NTPClient) ExportConfig() []byte {
	b, err := msgpack.Marshal(client)
	if err != nil {
		panic(err)
	}
	return b
}
