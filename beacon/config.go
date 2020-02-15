package beacon

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

// Config contains configuration about Beacon,
// use extra msgpack tag to hide raw field name.
type Config struct {
	Test Test `toml:"-" msgpack:"-"`

	Logger struct {
		Level     string `toml:"level"      msgpack:"a"`
		QueueSize int    `toml:"queue_size" msgpack:"b"`

		// if false, use ioutil.Discard, if true, use os.Stdout,
		// usually enable it for debug.
		Stdout bool      `toml:"stdout"     msgpack:"c"`
		Writer io.Writer `toml:"-"          msgpack:"-"`
	} `toml:"logger" msgpack:"aa"`

	Global struct {
		DNSCacheExpire      time.Duration `toml:"dns_cache_expire"      msgpack:"a"`
		TimeSyncSleepFixed  uint          `toml:"timesync_sleep_fixed"  msgpack:"b"`
		TimeSyncSleepRandom uint          `toml:"timesync_sleep_random" msgpack:"c"`
		TimeSyncInterval    time.Duration `toml:"timesync_interval"     msgpack:"d"`

		// generate from Controller
		Certificates      [][]byte                    `toml:"-" msgpack:"w"`
		ProxyClients      []*proxy.Client             `toml:"-" msgpack:"x"`
		DNSServers        map[string]*dns.Server      `toml:"-" msgpack:"y"`
		TimeSyncerClients map[string]*timesync.Client `toml:"-" msgpack:"z"`
	} `toml:"global" msgpack:"bb"`

	Client struct {
		ProxyTag string        `toml:"proxy_tag" msgpack:"a"`
		Timeout  time.Duration `toml:"timeout"   msgpack:"b"`
		DNSOpts  dns.Options   `toml:"dns"       msgpack:"c"`
	} `toml:"client" msgpack:"cc"`

	Register struct {
		// about failed to register
		SleepFixed  uint `toml:"sleep_fixed"  msgpack:"a"`
		SleepRandom uint `toml:"sleep_random" msgpack:"b"`

		// generate from Controller
		FirstBoot []byte `toml:"-" msgpack:"w"` // type: *messages.Bootstrap
		FirstKey  []byte `toml:"-" msgpack:"x"` // decrypt the first bootstrap data, AES CBC
		RestBoots []byte `toml:"-" msgpack:"y"` // type: []*messages.Bootstrap
		RestKey   []byte `toml:"-" msgpack:"z"` // decrypt rest bootstraps data, AES CBC
	} `toml:"register" msgpack:"dd"`

	Sender struct {
		MaxConns      int           `toml:"max_conns"       msgpack:"a"`
		Worker        int           `toml:"worker"          msgpack:"b"`
		Timeout       time.Duration `toml:"timeout"         msgpack:"c"`
		QueueSize     int           `toml:"queue_size"      msgpack:"d"`
		MaxBufferSize int           `toml:"max_buffer_size" msgpack:"e"`
		Interactive   bool          `toml:"interactive"     msgpack:"f"`
	} `toml:"sender" msgpack:"ee"`

	Syncer struct {
		ExpireTime time.Duration `toml:"expire_time" msgpack:"a"`
	} `toml:"syncer" msgpack:"ff"`

	Worker struct {
		Number        int `toml:"number"          msgpack:"a"`
		QueueSize     int `toml:"queue_size"      msgpack:"b"`
		MaxBufferSize int `toml:"max_buffer_size" msgpack:"c"`
	} `toml:"worker" msgpack:"gg"`

	Driver struct {
		// about query message from Controller
		SleepFixed  uint `toml:"sleep_fixed"  msgpack:"a"`
		SleepRandom uint `toml:"sleep_random" msgpack:"b"`
	} `toml:"driver" msgpack:"hh"`

	// generate from Controller
	Ctrl struct {
		KexPublicKey []byte `msgpack:"x"` // key exchange curve25519
		PublicKey    []byte `msgpack:"y"` // verify message ed25519
		BroadcastKey []byte `msgpack:"z"` // decrypt broadcast, key + iv
	} `toml:"-" msgpack:"ii"`

	// about service
	Service struct {
		Name        string `toml:"name"         msgpack:"a"`
		DisplayName string `toml:"display_name" msgpack:"b"`
		Description string `toml:"description"  msgpack:"c"`
	} `toml:"service" msgpack:"jj"`
}

// TestOptions include test options.
type TestOptions struct {
	Domain     string        `toml:"domain"` // about Beacon.global.DNSClient.TestServers()
	DNSOptions dns.Options   `toml:"dns"`
	Timeout    time.Duration `toml:"timeout"` // Beacon running timeout
}

// Run is used to create a Beacon with current configuration and run it to check error.
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
	beacon, err := New(cfg)
	if err != nil {
		return
	}
	defer beacon.Exit(nil)

	line := "------------------------------certificates--------------------------------\n"
	_, _ = output.Write([]byte(line))
	cfg.Certificates(output, beacon)
	line = "------------------------------proxy clients-------------------------------\n"
	_, _ = output.Write([]byte(line))
	cfg.ProxyClients(output, beacon)
	line = "-------------------------------DNS servers--------------------------------\n"
	_, _ = output.Write([]byte(line))
	_, err = cfg.DNSServers(ctx, output, beacon, opts)
	if err != nil {
		return
	}
	line = "---------------------------time syncer clients----------------------------\n"
	_, _ = output.Write([]byte(line))
	_, err = cfg.TimeSyncerClients(ctx, output, beacon)
	if err != nil {
		return
	}
	line = "-----------------------------beacon running-------------------------------\n"
	_, _ = output.Write([]byte(line))
	err = cfg.wait(ctx, beacon, opts.Timeout)
	return
}

// Certificates is used to print certificates.
func (cfg *Config) Certificates(writer io.Writer, beacon *Beacon) string {
	// set output
	var output io.Writer
	buf := new(bytes.Buffer)
	if writer != nil {
		output = io.MultiWriter(writer, buf)
	} else {
		output = buf
	}

	// print certificates
	for i, c := range beacon.global.Certificates() {
		_, _ = fmt.Fprintf(output, "ID: %d\n%s\n\n", i+1, cert.Print(c))
	}
	return buf.String()
}

// ProxyClients is used to print proxy clients.
func (cfg *Config) ProxyClients(writer io.Writer, beacon *Beacon) string {
	// set output
	var output io.Writer
	buf := new(bytes.Buffer)
	if writer != nil {
		output = io.MultiWriter(writer, buf)
	} else {
		output = buf
	}

	// print proxy clients
	for tag, client := range beacon.global.ProxyClients() {
		if tag == "" || tag == "direct" { // skip builtin proxy client
			continue
		}
		const format = "tag: %-10s mode: %-7s network: %-3s address: %s\n"
		_, _ = fmt.Fprintf(output, format, tag, client.Mode, client.Network, client.Address)
	}
	return buf.String()
}

// DNSServers is used to print and test DNS servers,
// if tests passed, show resolved IP addresses.
func (cfg *Config) DNSServers(
	ctx context.Context,
	writer io.Writer,
	beacon *Beacon,
	opts *TestOptions,
) (string, error) {
	// set output
	var output io.Writer
	buf := new(bytes.Buffer)
	if writer != nil {
		output = io.MultiWriter(writer, buf)
	} else {
		output = buf
	}

	// print DNS servers
	for tag, server := range beacon.global.DNSServers() {
		const format = "tag: %-14s skip: %-5t method: %-3s address: %s\n"
		_, _ = fmt.Fprintf(output, format, tag, server.SkipTest, server.Method, server.Address)
	}

	// add certificates to opts.DNSOptions about TLS
	certs := beacon.global.CertificatePEMs()
	// about TLS
	tCerts := opts.DNSOptions.TLSConfig.RootCAs
	tCerts = append(tCerts, certs...)
	opts.DNSOptions.TLSConfig.RootCAs = tCerts
	// about http.Transport TLS
	ttCerts := opts.DNSOptions.Transport.TLSClientConfig.RootCAs
	ttCerts = append(ttCerts, certs...)
	opts.DNSOptions.Transport.TLSClientConfig.RootCAs = ttCerts

	// resolve domain name
	result, err := beacon.global.DNSClient.TestServers(ctx, opts.Domain, &opts.DNSOptions)
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

// TimeSyncerClients is used to print and test time syncer clients,
// if tests passed, show the current time.
func (cfg *Config) TimeSyncerClients(
	ctx context.Context,
	writer io.Writer,
	beacon *Beacon,
) (string, error) {
	// set output
	var output io.Writer
	buf := new(bytes.Buffer)
	if writer != nil {
		output = io.MultiWriter(writer, buf)
	} else {
		output = buf
	}

	// print time syncer clients
	for tag, client := range beacon.global.TimeSyncerClients() {
		const format = "tag: %-10s skip: %-5t mode: %-4s\n"
		_, _ = fmt.Fprintf(output, format, tag, client.SkipTest, client.Mode)
	}

	// test syncer clients
	err := beacon.global.TimeSyncer.Test(ctx)
	if err != nil {
		return buf.String(), err
	}
	now := beacon.global.Now().Local().Format(logger.TimeLayout)
	_, _ = fmt.Fprintf(output, "\ncurrent time: %s\n", now)
	return buf.String(), nil
}

func (cfg *Config) wait(ctx context.Context, beacon *Beacon, timeout time.Duration) error {
	errChan := make(chan error, 1)
	go func() {
		var err error
		defer func() {
			if r := recover(); r != nil {
				err = xpanic.Error(r, "Config.wait")
			}
			errChan <- err
		}()
		err = beacon.Main()
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
		beacon.Exit(nil)
		return errors.New("beacon running timeout")
	case <-ctx.Done():
		beacon.Exit(nil)
		return ctx.Err()
	}
}

// Build is used to build configuration.
func (cfg *Config) Build() ([]byte, error) {
	return msgpack.Marshal(cfg)
}

// Load is used to load built configuration.
func (cfg *Config) Load(built []byte) error {
	return msgpack.Unmarshal(built, &cfg)
}
