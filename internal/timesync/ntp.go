package timesync

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/pelletier/go-toml"

	"project/internal/dns"
	"project/internal/proxy"
	"project/internal/timesync/ntp"
)

type NTP struct {
	proxyPool *proxy.Pool
	dnsClient *dns.Client

	Network string        `toml:"network"`
	Address string        `toml:"address"`
	Timeout time.Duration `toml:"timeout"`
	Version int           `toml:"version"`
	DNSOpts dns.Options   `toml:"dns"`
}

// NewNTP is used to create NTP client
func NewNTP(pool *proxy.Pool, client *dns.Client) *NTP {
	return &NTP{
		proxyPool: pool,
		dnsClient: client,
	}
}

// Query is used to query time
func (n *NTP) Query() (now time.Time, optsErr bool, err error) {
	// check network
	switch n.Network {
	case "", "udp", "udp4", "udp6":
	default:
		optsErr = true
		err = fmt.Errorf("unknown network: %s", n.Network)
		return
	}
	// check address
	host, port, err := net.SplitHostPort(n.Address)
	if err != nil {
		optsErr = true
		return
	}
	// set NTP options
	ntpOpts := ntp.Options{
		Network: n.Network,
		Timeout: n.Timeout,
		Version: n.Version,
	}
	if ntpOpts.Network == "" {
		ntpOpts.Network = "udp"
	}

	// set proxy
	p, err := n.proxyPool.Get("")
	// support udp proxy future
	/*
		if err != nil {
			optsErr = true
			return
		}
	*/
	ntpOpts.Dial = p.Dial

	// resolve domain name
	dnsOptsCopy := n.DNSOpts
	result, err := n.dnsClient.Resolve(host, &dnsOptsCopy)
	if err != nil {
		optsErr = true
		err = fmt.Errorf("resolve domain name failed: %s", err)
		return
	}

	// query NTP server
	var resp *ntp.Response
	for i := 0; i < len(result); i++ {
		resp, err = ntp.Query(net.JoinHostPort(result[i], port), &ntpOpts)
		if err == nil {
			now = resp.Time
			return
		}
	}
	err = errors.New("query ntp server failed")
	return
}

func (n *NTP) Import(b []byte) error {
	return toml.Unmarshal(b, n)
}

func (n *NTP) Export() []byte {
	b, _ := toml.Marshal(n)
	return b
}

// TestNTP is used to create a NTP client to test toml config
func TestNTP(config []byte) error {
	return new(NTP).Import(config)
}
