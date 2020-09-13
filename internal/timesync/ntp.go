package timesync

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/pkg/errors"

	"project/external/ntp"

	"project/internal/dns"
	"project/internal/patch/toml"
	"project/internal/proxy"
)

// NTP is used to create a NTP client to synchronize time.
type NTP struct {
	ctx       context.Context
	proxyPool *proxy.Pool
	dnsClient *dns.Client

	Network string        `toml:"network"`
	Address string        `toml:"address"`
	Timeout time.Duration `toml:"timeout"`
	Version int           `toml:"version"`
	DNSOpts dns.Options   `toml:"dns" check:"-"`
}

// NewNTP is used to create a NTP client.
func NewNTP(ctx context.Context, proxyPool *proxy.Pool, dnsClient *dns.Client) *NTP {
	return &NTP{
		ctx:       ctx,
		proxyPool: proxyPool,
		dnsClient: dnsClient,
	}
}

// Query is used to query time from NTP server.
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
	proxyClient, _ := n.proxyPool.Get("")
	// support udp proxy in the future
	/*
		if err != nil {
			optsErr = true
			return
		}
	*/
	ntpOpts.Dial = proxyClient.Dial

	// resolve domain name
	result, err := n.dnsClient.ResolveContext(n.ctx, host, &n.DNSOpts)
	if err != nil {
		optsErr = true
		return
	}

	// query NTP server
	var resp *ntp.Response
	for i := 0; i < len(result); i++ {
		address := net.JoinHostPort(result[i], port)
		resp, err = ntp.Query(address, &ntpOpts)
		if err == nil {
			break
		}
	}
	if err == nil {
		now = resp.Time
		return
	}
	err = errors.Errorf("failed to query ntp server: %s", err)
	return
}

// Import is used to import configuration from toml and check.
func (n *NTP) Import(cfg []byte) error {
	err := toml.Unmarshal(cfg, n)
	if err != nil {
		return err
	}
	if n.Address == "" {
		return errors.New("empty address")
	}
	_, _, err = net.SplitHostPort(n.Address)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// Export is used to export current configuration to toml.
func (n *NTP) Export() []byte {
	cfg, _ := toml.Marshal(n)
	return cfg
}

// TestNTP is used to create a NTP client to test toml config.
func TestNTP(config []byte) error {
	return new(NTP).Import(config)
}
