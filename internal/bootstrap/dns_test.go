package bootstrap

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/dns"
	"project/internal/patch/toml"
	"project/internal/testsuite"
	"project/internal/testsuite/testdns"
	"project/internal/xnet"
)

func TestDNS(t *testing.T) {
	dnsClient, _, proxyMgr, _ := testdns.DNSClient(t)
	defer func() { require.NoError(t, proxyMgr.Close()) }()

	if testsuite.IPv4Enabled {
		listeners := []*Listener{{
			Mode:    xnet.ModeTLS,
			Network: "tcp",
			Address: "127.0.0.1:443",
		}}
		DNS := NewDNS(context.Background(), nil)
		DNS.Host = "localhost"
		DNS.Mode = xnet.ModeTLS
		DNS.Network = "tcp"
		DNS.Port = "443"
		DNS.Options.Mode = dns.ModeSystem
		DNS.Options.Type = dns.TypeIPv4
		b, err := DNS.Marshal()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, DNS)

		DNS = NewDNS(context.Background(), dnsClient)
		err = DNS.Unmarshal(b)
		require.NoError(t, err)

		for i := 0; i < 10; i++ {
			resolved, err := DNS.Resolve()
			require.NoError(t, err)
			resolved = testDecryptListeners(resolved)
			require.Equal(t, listeners, resolved)
		}

		testsuite.IsDestroyed(t, DNS)
	}

	if testsuite.IPv6Enabled {
		listeners := []*Listener{{
			Mode:    xnet.ModeTLS,
			Network: "tcp",
			Address: "[::1]:443",
		}}
		DNS := NewDNS(context.Background(), nil)
		DNS.Host = "localhost"
		DNS.Mode = xnet.ModeTLS
		DNS.Network = "tcp"
		DNS.Port = "443"
		DNS.Options.Mode = dns.ModeSystem
		DNS.Options.Type = dns.TypeIPv6
		b, err := DNS.Marshal()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, DNS)

		DNS = NewDNS(context.Background(), dnsClient)
		err = DNS.Unmarshal(b)
		require.NoError(t, err)

		for i := 0; i < 10; i++ {
			resolved, err := DNS.Resolve()
			require.NoError(t, err)
			resolved = testDecryptListeners(resolved)
			require.Equal(t, listeners, resolved)
		}

		testsuite.IsDestroyed(t, DNS)
	}
}

func TestDNS_Validate(t *testing.T) {
	DNS := NewDNS(context.Background(), nil)
	require.EqualError(t, DNS.Validate(), "empty host")

	// invalid domain name
	DNS.Host = "1.1.1.1"
	require.Error(t, DNS.Validate())

	DNS.Host = "localhost"

	// mismatched mode and network
	DNS.Mode = xnet.ModeTLS
	DNS.Network = "udp"
	require.Error(t, DNS.Validate())
	DNS.Network = "tcp"

	// invalid port
	b, err := DNS.Marshal()
	require.Error(t, err)
	require.Nil(t, b)
}

func TestDNS_Unmarshal(t *testing.T) {
	DNS := NewDNS(context.Background(), nil)

	// unmarshal invalid config
	require.Error(t, DNS.Unmarshal([]byte{0x00}))

	// with incorrect config
	require.Error(t, DNS.Unmarshal(nil))
}

func TestDNS_Resolve(t *testing.T) {
	dnsClient, _, proxyMgr, _ := testdns.DNSClient(t)
	defer func() { require.NoError(t, proxyMgr.Close()) }()

	DNS := NewDNS(context.Background(), dnsClient)
	config := []byte(`
         host    = "localhost"
         mode    = "tls"
         network = "tcp"
         port    = "443"
         
         [options]
           mode = "foo mode"  `)
	require.NoError(t, DNS.Unmarshal(config))

	if testsuite.IPv4Enabled {
		listeners, err := DNS.Resolve()
		require.Error(t, err)
		require.Nil(t, listeners)
	}

	if testsuite.IPv6Enabled {
		listeners, err := DNS.Resolve()
		require.Error(t, err)
		require.Nil(t, listeners)
	}
}

func TestDNSPanic(t *testing.T) {
	t.Run("no CBC", func(t *testing.T) {
		DNS := NewDNS(context.Background(), nil)

		func() {
			defer func() {
				r := recover()
				require.NotNil(t, r)
				t.Log(r)
			}()
			_, _ = DNS.Resolve()
		}()

		testsuite.IsDestroyed(t, DNS)
	})

	t.Run("invalid options", func(t *testing.T) {
		DNS := NewDNS(context.Background(), nil)

		func() {
			var err error
			key := bytes.Repeat([]byte{0}, aes.Key128Bit)
			DNS.cbc, err = aes.NewCBC(key, key)
			require.NoError(t, err)
			enc, err := DNS.cbc.Encrypt(testsuite.Bytes())
			require.NoError(t, err)
			DNS.enc = enc

			defer func() {
				r := recover()
				require.NotNil(t, r)
				t.Log(r)
			}()
			_, _ = DNS.Resolve()
		}()

		testsuite.IsDestroyed(t, DNS)
	})
}

func TestDNSOptions(t *testing.T) {
	config, err := ioutil.ReadFile("testdata/dns.toml")
	require.NoError(t, err)
	DNS := NewDNS(context.Background(), nil)
	require.NoError(t, toml.Unmarshal(config, DNS))
	require.NoError(t, DNS.Validate())

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "localhost", actual: DNS.Host},
		{expected: xnet.ModeTLS, actual: DNS.Mode},
		{expected: "tcp", actual: DNS.Network},
		{expected: "443", actual: DNS.Port},
		{expected: dns.ModeSystem, actual: DNS.Options.Mode},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}
}
