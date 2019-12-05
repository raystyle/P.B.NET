package node

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/vmihailenco/msgpack/v4"

	"project/internal/crypto/cert"
	"project/internal/dns"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/timesync"
)

// Config include configuration about Node
type Config struct {
	Debug Debug `toml:"-" msgpack:"-"`

	// CheckMode is used to check whether
	// the configuration is correct
	CheckMode bool `toml:"-" msgpack:"-"`

	Logger struct {
		Level     string    `toml:"level"`
		QueueSize int       `toml:"queue_size"`
		Writer    io.Writer `toml:"-" msgpack:"-"` // for check config
	} `toml:"logger"`

	Global struct {
		DNSCacheExpire   time.Duration `toml:"dns_cache_expire"`
		TimeSyncInterval time.Duration `toml:"time_sync_interval"`

		// generate from controller
		Certificates      [][]byte                    `toml:"-"`
		ProxyClients      []*proxy.Client             `toml:"-"`
		DNSServers        map[string]*dns.Server      `toml:"-"`
		TimeSyncerClients map[string]*timesync.Client `toml:"-"`
	} `toml:"global"`

	Client struct { // options
		ProxyTag string        `toml:"proxy_tag"`
		Timeout  time.Duration `toml:"timeout"`
		DNSOpts  dns.Options   `toml:"dns"`
	} `toml:"client"`

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
		Timeout       time.Duration `toml:"timeout"`
		QueueSize     int           `toml:"queue_size"`
		MaxBufferSize int           `toml:"max_buffer_size"`
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

		// generate from controller
		AESCrypto []byte `toml:"-"`
		Listeners []byte `toml:"-"`
	} `toml:"server"`

	// generate from controller
	CTRL struct {
		ExPublicKey  []byte // key exchange curve25519
		PublicKey    []byte // verify message ed25519
		BroadcastKey []byte // decrypt broadcast, key + iv
	} `toml:"-"`
}

// CheckOptions include options about check configuration
type CheckOptions struct {
	Domain     string        `toml:"domain"`
	DNSOptions dns.Options   `toml:"dns"`
	Timeout    time.Duration `toml:"timeout"`
	Writer     io.Writer     `toml:"-"`
}

// Check is used to check node configuration
func (cfg *Config) Check(ctx context.Context, opts *CheckOptions) (output *bytes.Buffer, err error) {
	output = new(bytes.Buffer)

	var writer io.Writer
	if opts.Writer == nil {
		writer = output
	} else {
		writer = io.MultiWriter(output, opts.Writer)
	}

	defer func() {
		if err != nil {
			_, _ = fmt.Fprintln(writer, "\ntests failed:", err)
		}
	}()

	cfg.CheckMode = true
	cfg.Logger.Writer = writer

	// create Node
	node, err := New(cfg)
	if err != nil {
		return
	}
	defer node.Exit(nil)

	// print certificates
	line := "------------------------------certificates--------------------------------\n"
	_, _ = writer.Write([]byte(line))
	for i, c := range node.global.Certificates() {
		_, _ = fmt.Fprintf(writer, "ID: %d\n%s\n\n", i+1, cert.Print(c))
	}

	// print proxy clients
	line = "------------------------------proxy clients-------------------------------\n"
	_, _ = writer.Write([]byte(line))
	for tag, client := range node.global.ProxyClients() {
		// skip builtin proxy client
		if tag == "" || tag == "direct" {
			continue
		}
		const format = "tag: %-10s mode: %-7s network: %-3s address: %s\n"
		_, _ = fmt.Fprintf(writer, format, tag, client.Mode, client.Network, client.Address)
	}

	// test DNS client
	line = "-------------------------------DNS clients--------------------------------\n"
	_, _ = writer.Write([]byte(line))
	// print DNS servers
	for tag, server := range node.global.DNSServers() {
		const format = "tag: %-14s skip: %-5t method: %-3s address: %s\n"
		_, _ = fmt.Fprintf(writer, format, tag, server.SkipTest, server.Method, server.Address)
	}
	domain := opts.Domain
	if domain == "" {
		domain = "github.com"
	}
	// add certificates
	certs := node.global.CertificatePEMs()
	tCerts := opts.DNSOptions.TLSConfig.RootCAs
	opts.DNSOptions.TLSConfig.RootCAs = append(tCerts, certs...)
	ttCerts := opts.DNSOptions.Transport.TLSClientConfig.RootCAs
	opts.DNSOptions.Transport.TLSClientConfig.RootCAs = append(ttCerts, certs...)
	result, err := node.global.dnsClient.TestServers(ctx, domain, &opts.DNSOptions)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(writer, "\ntest domain: %s result: %s\n", domain, result)

	// test time syncer
	line = "---------------------------time syncer clients----------------------------\n"
	_, _ = writer.Write([]byte(line))
	// print time syncer clients
	for tag, client := range node.global.TimeSyncerClients() {
		const format = "tag: %-10s skip: %-5t mode: %-4s\n"
		_, _ = fmt.Fprintf(writer, format, tag, client.SkipTest, client.Mode)
	}
	err = node.global.timeSyncer.Test()
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(writer, "\ntime: %s\n", node.global.Now().Format(logger.TimeLayout))

	// run Node
	line = "-------------------------------node logger-------------------------------\n"
	_, _ = writer.Write([]byte(line))
	errChan := make(chan error)
	go func() {
		errChan <- node.Main()
	}()
	node.Wait()
	timeout := opts.Timeout
	if timeout < 1 {
		timeout = 15 * time.Second
	}
	select {
	case err = <-errChan:
		return
	case <-time.After(timeout):
		node.Exit(nil)
		err = errors.New("check timeout")
		return
	}
}

// Build is used to build node configuration
func (cfg *Config) Build() ([]byte, error) {
	return msgpack.Marshal(cfg)
}

// Debug is used to test
type Debug struct {
	// skip sync time
	SkipTimeSyncer bool

	// from controller
	Broadcast chan []byte
	Send      chan []byte
}

// copy Config.Client
type opts struct {
	ProxyTag string
	Timeout  time.Duration
	DNSOpts  dns.Options
}
