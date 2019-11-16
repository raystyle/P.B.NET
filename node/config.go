package node

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/vmihailenco/msgpack/v4"

	"project/internal/dns"
	"project/internal/messages"
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

	LogLevel string `toml:"log_level"`

	Global struct {
		DNSCacheExpire   time.Duration `toml:"dns_cache_expire"`
		TimeSyncInterval time.Duration `toml:"time_sync_interval"`

		ProxyClients      map[string]*proxy.Client    `toml:"-"`
		DNSServers        map[string]*dns.Server      `toml:"-"`
		TimeSyncerClients map[string]*timesync.Client `toml:"-"`
	} `toml:"global"`

	Sender struct {
		MaxBufferSize int `toml:"max_buffer_size"`
		Worker        int `toml:"worker"`
		QueueSize     int `toml:"queue_size"`
	} `toml:"sender"`

	Syncer struct {
		MaxConns      int           `toml:"max_conns"`
		MaxBufferSize int           `toml:"max_buffer_size"`
		Worker        int           `toml:"worker"`
		QueueSize     int           `toml:"queue_size"`
		ExpireTime    time.Duration `toml:"expire_time"`
	} `toml:"syncer"`

	Register struct {
		Bootstraps []*messages.Bootstrap `toml:"-"`
	} `toml:"register"`

	Server struct {
		MaxConns int `toml:"max_conns"` // single listener
		// key = tag
		Listeners []*messages.Listener `toml:"-"`
	} `toml:"server"`

	// controller configs
	CTRL struct {
		PublicKey   []byte // ed25519
		ExPublicKey []byte // curve25519
		AESCrypto   []byte // key + iv
	} `toml:"-"`
}

// before create a node need check config
func (cfg *Config) Check(ctx context.Context, domain string) (*bytes.Buffer, error) {
	cfg.CheckMode = true
	node, err := New(cfg)
	if err != nil {
		return nil, err
	}
	defer node.Exit(nil)

	// test DNS client
	output := new(bytes.Buffer)
	output.WriteString("----------------------DNS client-----------------------")
	// print DNS servers
	for tag, server := range node.global.dnsClient.Servers() {
		const format = "tag: %s method: %s address: %s skip test: %t"
		_, _ = fmt.Fprintf(output, format, tag, server.Method, server.Address, server.SkipTest)
	}
	result, err := node.global.dnsClient.TestServers(ctx, domain, new(dns.Options))
	if err != nil {
		return nil, err
	}
	_, _ = fmt.Fprintf(output, "test domain: %s, result: %s", domain, result)

	// test time syncer
	output.WriteString("----------------------time syncer-----------------------")
	err = node.global.timeSyncer.Test()
	if err != nil {
		return output, err
	}
	return output, nil
}

func (cfg *Config) Build() ([]byte, error) {
	return msgpack.Marshal(cfg)
}
