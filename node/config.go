package node

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/vmihailenco/msgpack/v4"

	"project/internal/dns"
	"project/internal/proxy"
	"project/internal/timesync"
)

type Debug struct {
	// skip sync time
	SkipTimeSyncer bool

	// from controller
	Broadcast chan []byte
	Send      chan []byte
}

type Config struct {
	Debug Debug `toml:"-"`

	// CheckMode is used to check whether
	// the configuration is correct
	CheckMode bool `toml:"-"`

	Logger struct {
		Level  string    `toml:"level"`
		Writer io.Writer `toml:"-"` // for check config
	} `toml:"logger"`

	Global struct {
		DNSCacheExpire   time.Duration `toml:"dns_cache_expire"`
		TimeSyncInterval time.Duration `toml:"time_sync_interval"`

		// generate configs from controller
		ProxyClients      []*proxy.Client             `toml:"-"`
		DNSServers        map[string]*dns.Server      `toml:"-"`
		TimeSyncerClients map[string]*timesync.Client `toml:"-"`
	} `toml:"global"`

	Register struct {

		// generate configs from controller
		Bootstraps []byte `toml:"-"`
	} `toml:"register"`

	Forwarder struct {
		MaxCtrlConns   int `toml:"max_ctrl_conns"`
		MaxNodeConns   int `toml:"max_node_conns"`
		MaxBeaconConns int `toml:"max_beacon_conns"`
	} `toml:"forwarder"`

	Sender struct {
		Worker        int           `toml:"worker"`
		QueueSize     int           `toml:"queue_size"`
		MaxBufferSize int           `toml:"max_buffer_size"`
		Timeout       time.Duration `toml:"timeout"`
		MaxConns      int           `toml:"max_conns"`
	} `toml:"sender"`

	Syncer struct {
		ExpireTime time.Duration `toml:"expire_time"`
	} `toml:"syncer"`

	Worker struct {
		Number        int `toml:"number"`
		QueueSize     int `toml:"queue_size"`
		MaxBufferSize int `toml:"max_buffer_size"`
	} `toml:"worker"`

	Server struct {
		MaxConns int           `toml:"max_conns"` // single listener
		Timeout  time.Duration `toml:"timeout"`   // handshake timeout

		// generate configs from controller
		AESCrypto []byte `toml:"-"` // decrypt listeners data
		Listeners []byte `toml:"-"`
	} `toml:"server"`

	// generate configs from controller
	CTRL struct {
		ExPublicKey []byte // key exchange curve25519
		PublicKey   []byte // verify message ed25519
		AESCrypto   []byte // decrypt broadcast, key + iv
	} `toml:"-"`
}

type CheckOptions struct {
	Domain     string
	DNSOptions dns.Options
	Timeout    time.Duration
}

// before create a node need check config
func (cfg *Config) Check(ctx context.Context, opts *CheckOptions) (output *bytes.Buffer, err error) {
	if opts == nil {
		opts = new(CheckOptions)
	}

	output = new(bytes.Buffer)
	defer func() {
		if err != nil {
			_, _ = fmt.Fprintln(output, err)
		}
	}()
	cfg.CheckMode = true
	cfg.Logger.Writer = output

	// create Node
	node, err := New(cfg)
	if err != nil {
		return
	}
	defer node.Exit(nil)

	// print proxy clients
	pLine := "------------------------------proxy client-------------------------------"
	output.WriteString(pLine)
	for tag, client := range node.global.proxyPool.Clients() {
		const format = "tag: %-10s mode: %-7s network: %-3s address: %s"
		_, _ = fmt.Fprintf(output, format, tag, client.Mode, client.Network, client.Address)
	}

	// test DNS client
	dLine := "-------------------------------DNS client--------------------------------"
	output.WriteString(dLine)
	// print DNS servers
	for tag, server := range node.global.dnsClient.Servers() {
		const format = "tag: %-10s skip test: %t method: %-3s address: %s"
		_, _ = fmt.Fprintf(output, format, tag, server.SkipTest, server.Method, server.Address)
	}
	domain := opts.Domain
	if domain == "" {
		domain = "github.com"
	}
	result, err := node.global.dnsClient.TestServers(ctx, domain, &opts.DNSOptions)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(output, "test domain: %s, result: %s", domain, result)

	// test time syncer
	tLine := "-------------------------------time syncer-------------------------------"
	output.WriteString(tLine)
	err = node.global.timeSyncer.Test()
	if err != nil {
		return
	}

	// run Node
	errChan := make(chan error)
	go func() {
		errChan <- node.Main()
	}()
	node.TestWait()
	select {
	case err = <-errChan:
		return
	case <-time.After(opts.Timeout):
		node.Exit(nil)
		err = errors.New("check timeout")
		return
	}
}

func (cfg *Config) Build() ([]byte, error) {
	return msgpack.Marshal(cfg)
}
