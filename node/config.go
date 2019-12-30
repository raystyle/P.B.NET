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
	"project/internal/xpanic"
)

// Test contains test data
type Test struct {
	// Node.Main()
	SkipSynchronizeTime bool

	// test messages from controller
	BroadcastTestMsg chan []byte
	SendTestMsg      chan []byte
}

// Config contains configuration about Node
type Config struct {
	Test Test `toml:"-" msgpack:"-"`

	Logger struct {
		Level     string    `toml:"level"      msgpack:"a"`
		QueueSize int       `toml:"queue_size" msgpack:"b"`
		Writer    io.Writer `toml:"-"          msgpack:"-"`
	} `toml:"logger" msgpack:"aa"`

	Global struct {
		DNSCacheExpire      time.Duration `toml:"dns_cache_expire"      msgpack:"a"`
		TimeSyncSleepFixed  int           `toml:"timesync_sleep_fixed"  msgpack:"b"`
		TimeSyncSleepRandom int           `toml:"timesync_sleep_random" msgpack:"c"`
		TimeSyncInterval    time.Duration `toml:"timesync_interval"     msgpack:"d"`

		// generate from controller
		Certificates      [][]byte                    `toml:"-" msgpack:"w"`
		ProxyClients      []*proxy.Client             `toml:"-" msgpack:"x"`
		DNSServers        map[string]*dns.Server      `toml:"-" msgpack:"y"`
		TimeSyncerClients map[string]*timesync.Client `toml:"-" msgpack:"z"`
	} `toml:"global" msgpack:"bb"`

	Client cOpts `toml:"client" msgpack:"cc"`

	Register struct {
		Skip bool `toml:"skip" msgpack:"a"` // skip register for genesis node

		// generate configs from controller
		Bootstraps []byte `toml:"-" msgpack:"b"`
	} `toml:"register" msgpack:"dd"`

	Forwarder struct {
		MaxCtrlConns   int `toml:"max_ctrl_conns"   msgpack:"a"`
		MaxNodeConns   int `toml:"max_node_conns"   msgpack:"b"`
		MaxBeaconConns int `toml:"max_beacon_conns" msgpack:"c"`
	} `toml:"forwarder" msgpack:"ee"`

	Sender struct {
		Worker        int           `toml:"worker"          msgpack:"a"`
		Timeout       time.Duration `toml:"timeout"         msgpack:"b"`
		QueueSize     int           `toml:"queue_size"      msgpack:"c"`
		MaxBufferSize int           `toml:"max_buffer_size" msgpack:"d"`
	} `toml:"sender" msgpack:"ff"`

	Syncer struct {
		ExpireTime time.Duration `toml:"expire_time" msgpack:"a"`
	} `toml:"syncer" msgpack:"gg"`

	Worker struct {
		Number        int `toml:"number"          msgpack:"a"`
		QueueSize     int `toml:"queue_size"      msgpack:"b"`
		MaxBufferSize int `toml:"max_buffer_size" msgpack:"c"`
	} `toml:"worker" msgpack:"hh"`

	Server struct {
		MaxConns int           `toml:"max_conns" msgpack:"a"` // single listener
		Timeout  time.Duration `toml:"timeout"   msgpack:"b"` // handshake timeout

		// generate from controller
		AESCrypto []byte `toml:"-" msgpack:"y"` // decrypt Listeners data
		Listeners []byte `toml:"-" msgpack:"z"` // type: []*messages.Listener
	} `toml:"server" msgpack:"ii"`

	// generate from controller
	CTRL struct {
		ExPublicKey  []byte `msgpack:"x"` // key exchange curve25519
		PublicKey    []byte `msgpack:"y"` // verify message ed25519
		BroadcastKey []byte `msgpack:"z"` // decrypt broadcast, key + iv
	} `toml:"-" msgpack:"jj"`
}

// client options
type cOpts struct {
	ProxyTag string        `toml:"proxy_tag" msgpack:"a"`
	Timeout  time.Duration `toml:"timeout"   msgpack:"b"`
	DNSOpts  dns.Options   `toml:"dns"       msgpack:"c"`
}

// TestOptions include options about test
type TestOptions struct {
	// about node.global.DNSClient.TestServers()
	Domain     string      `toml:"domain"`
	DNSOptions dns.Options `toml:"dns"`

	// node run timeout
	Timeout time.Duration `toml:"timeout"`
}

// Run is used to create a node with current configuration and run it to check error
func (cfg *Config) Run(ctx context.Context, output io.Writer, opts *TestOptions) (err error) {
	defer func() {
		if err != nil {
			_, _ = fmt.Fprintln(output, "\ntests failed:", err)
		} else {
			_, _ = fmt.Fprintln(output, "\ntests passed")
		}
	}()
	cfg.Logger.Level = "debug"
	cfg.Logger.Writer = output
	node, err := New(cfg)
	if err != nil {
		return
	}
	defer node.Exit(nil)

	line := "------------------------------certificates--------------------------------\n"
	_, _ = output.Write([]byte(line))
	cfg.Certificates(output, node)
	line = "------------------------------proxy clients-------------------------------\n"
	_, _ = output.Write([]byte(line))
	cfg.ProxyClients(output, node)
	line = "-------------------------------DNS servers--------------------------------\n"
	_, _ = output.Write([]byte(line))
	_, err = cfg.DNSServers(ctx, output, node, opts)
	if err != nil {
		return
	}
	line = "---------------------------time syncer clients----------------------------\n"
	_, _ = output.Write([]byte(line))
	_, err = cfg.TimeSyncerClients(ctx, output, node)
	if err != nil {
		return
	}
	line = "------------------------------node running--------------------------------\n"
	_, _ = output.Write([]byte(line))
	err = cfg.wait(ctx, node, opts.Timeout)
	return
}

// Certificates is used to print certificates
func (cfg *Config) Certificates(writer io.Writer, node *Node) string {
	// set output
	var output io.Writer
	buf := bytes.NewBuffer(nil)
	if writer != nil {
		output = io.MultiWriter(writer, buf)
	} else {
		output = buf
	}

	// print certificates
	for i, c := range node.global.Certificates() {
		_, _ = fmt.Fprintf(output, "ID: %d\n%s\n\n", i+1, cert.Print(c))
	}
	return buf.String()
}

// ProxyClients is used to print proxy clients
func (cfg *Config) ProxyClients(writer io.Writer, node *Node) string {
	// set output
	var output io.Writer
	buf := bytes.NewBuffer(nil)
	if writer != nil {
		output = io.MultiWriter(writer, buf)
	} else {
		output = buf
	}

	// print proxy clients
	for tag, client := range node.global.ProxyClients() {
		if tag == "" || tag == "direct" { // skip builtin proxy client
			continue
		}
		const format = "tag: %-10s mode: %-7s network: %-3s address: %s\n"
		_, _ = fmt.Fprintf(output, format, tag, client.Mode, client.Network, client.Address)
	}
	return buf.String()
}

// DNSServers is used to print and test DNS servers
// if tests passed, show resolved ip
func (cfg *Config) DNSServers(
	ctx context.Context,
	writer io.Writer,
	node *Node,
	opts *TestOptions,
) (string, error) {
	// set output
	var output io.Writer
	buf := bytes.NewBuffer(nil)
	if writer != nil {
		output = io.MultiWriter(writer, buf)
	} else {
		output = buf
	}

	// print DNS servers
	for tag, server := range node.global.DNSServers() {
		const format = "tag: %-14s skip: %-5t method: %-3s address: %s\n"
		_, _ = fmt.Fprintf(output, format, tag, server.SkipTest, server.Method, server.Address)
	}

	// add certificates to opts.DNSOptions about TLS
	certs := node.global.CertificatePEMs()
	// about TLS
	tCerts := opts.DNSOptions.TLSConfig.RootCAs
	tCerts = append(tCerts, certs...)
	opts.DNSOptions.TLSConfig.RootCAs = tCerts
	// about http.Transport TLS
	ttCerts := opts.DNSOptions.Transport.TLSClientConfig.RootCAs
	ttCerts = append(ttCerts, certs...)
	opts.DNSOptions.Transport.TLSClientConfig.RootCAs = ttCerts

	// resolve domain name
	result, err := node.global.DNSClient.TestServers(ctx, opts.Domain, &opts.DNSOptions)
	if err != nil {
		return buf.String(), err
	}
	// print string slice
	var r string
	for i, s := range result {
		if i == 0 {
			r = s
		} else {
			r += ", " + s
		}
	}
	_, _ = fmt.Fprintf(output, "\ntest domain: %s\nresolved ip: %s\n", opts.Domain, r)
	return buf.String(), nil
}

// TimeSyncerClients is used to print and test time syncer clients
// if tests passed, show the current time
func (cfg *Config) TimeSyncerClients(
	ctx context.Context,
	writer io.Writer,
	node *Node,
) (string, error) {
	// set output
	var output io.Writer
	buf := bytes.NewBuffer(nil)
	if writer != nil {
		output = io.MultiWriter(writer, buf)
	} else {
		output = buf
	}

	// print time syncer clients
	for tag, client := range node.global.TimeSyncerClients() {
		const format = "tag: %-10s skip: %-5t mode: %-4s\n"
		_, _ = fmt.Fprintf(output, format, tag, client.SkipTest, client.Mode)
	}

	// test syncer clients
	err := node.global.TimeSyncer.Test(ctx)
	if err != nil {
		return buf.String(), err
	}
	now := node.global.Now().Format(logger.TimeLayout)
	_, _ = fmt.Fprintf(output, "\ncurrent time: %s\n", now)
	return buf.String(), nil
}

func (cfg *Config) wait(ctx context.Context, node *Node, timeout time.Duration) error {
	errChan := make(chan error, 1)
	go func() {
		var err error
		defer func() {
			if r := recover(); r != nil {
				err = xpanic.Error(r, "Config.wait")
			}
			errChan <- err
		}()
		err = node.Main()
	}()
	if timeout < 1 {
		timeout = 15 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-errChan:
		close(errChan)
		return err
	case <-timer.C:
		node.Exit(nil)
		return errors.New("node running timeout")
	case <-ctx.Done():
		node.Exit(nil)
		return ctx.Err()
	}
}

// Build is used to build node configuration
func (cfg *Config) Build() ([]byte, error) {
	return msgpack.Marshal(cfg)
}
