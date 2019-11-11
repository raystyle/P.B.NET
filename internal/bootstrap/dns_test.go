package bootstrap

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/dns"
	"project/internal/testsuite"
	"project/internal/testsuite/testdns"
	"project/internal/xnet"
)

func TestDNS(t *testing.T) {
	client, _, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()

	if testsuite.EnableIPv4() {
		nodes := []*Node{{
			Mode:    xnet.ModeTLS,
			Network: "tcp",
			Address: "127.0.0.1:443",
		}}
		DNS := NewDNS(nil, nil)
		DNS.Host = "localhost"
		DNS.Mode = xnet.ModeTLS
		DNS.Network = "tcp"
		DNS.Port = "443"
		DNS.Options.Mode = dns.ModeSystem
		DNS.Options.Type = dns.TypeIPv4
		b, err := DNS.Marshal()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, DNS)

		DNS = NewDNS(context.Background(), client)
		err = DNS.Unmarshal(b)
		require.NoError(t, err)

		for i := 0; i < 10; i++ {
			resolved, err := DNS.Resolve()
			require.NoError(t, err)
			require.Equal(t, nodes, resolved)
		}

		testsuite.IsDestroyed(t, DNS)
	}

	if testsuite.EnableIPv6() {
		nodes := []*Node{{
			Mode:    xnet.ModeTLS,
			Network: "tcp",
			Address: "[::1]:443",
		}}
		DNS := NewDNS(nil, nil)
		DNS.Host = "localhost"
		DNS.Mode = xnet.ModeTLS
		DNS.Network = "tcp"
		DNS.Port = "443"
		DNS.Options.Mode = dns.ModeSystem
		DNS.Options.Type = dns.TypeIPv6
		b, err := DNS.Marshal()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, DNS)

		DNS = NewDNS(context.Background(), client)
		err = DNS.Unmarshal(b)
		require.NoError(t, err)

		for i := 0; i < 10; i++ {
			resolved, err := DNS.Resolve()
			require.NoError(t, err)
			require.Equal(t, nodes, resolved)
		}

		testsuite.IsDestroyed(t, DNS)
	}
}

func TestDNS_Validate(t *testing.T) {
	DNS := NewDNS(nil, nil)
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
	DNS := NewDNS(nil, nil)

	// unmarshal invalid config
	require.Error(t, DNS.Unmarshal([]byte{0x00}))

	// with incorrect config
	require.Error(t, DNS.Unmarshal(nil))
}

func TestDNS_Resolve(t *testing.T) {
	client, _, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()

	DNS := NewDNS(context.Background(), client)
	config := []byte(`
         host    = "localhost"
         mode    = "tls"
         network = "tcp"
         port    = "443"
         
         [options]
           mode = "foo mode"  `)
	require.NoError(t, DNS.Unmarshal(config))

	if testsuite.EnableIPv4() {
		nodes, err := DNS.Resolve()
		require.Error(t, err)
		require.Nil(t, nodes)
	}

	if testsuite.EnableIPv6() {
		nodes, err := DNS.Resolve()
		require.Error(t, err)
		require.Nil(t, nodes)
	}
}

func TestDNSPanic(t *testing.T) {
	t.Parallel()

	t.Run("no CBC", func(t *testing.T) {
		DNS := NewDNS(nil, nil)
		defer testsuite.IsDestroyed(t, DNS)
		defer func() {
			r := recover()
			require.NotNil(t, r)
			t.Log(r)
		}()
		_, _ = DNS.Resolve()
	})

	t.Run("invalid options", func(t *testing.T) {
		DNS := NewDNS(nil, nil)
		defer testsuite.IsDestroyed(t, DNS)
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
	})
}

func TestDNSOptions(t *testing.T) {
	config, err := ioutil.ReadFile("testdata/dns.toml")
	require.NoError(t, err)
	DNS := NewDNS(nil, nil)
	require.NoError(t, toml.Unmarshal(config, DNS))

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
