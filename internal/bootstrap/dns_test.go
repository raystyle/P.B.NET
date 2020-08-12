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

func TestDNS_Validate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	DNS := DNS{}

	t.Run("empty host", func(t *testing.T) {
		err := DNS.Validate()
		require.EqualError(t, err, "empty host")
	})

	t.Run("invalid domain name", func(t *testing.T) {
		DNS.Host = "1.1.1.1"
		defer func() { DNS.Host = "localhost" }()

		err := DNS.Validate()
		require.EqualError(t, err, "invalid domain name: 1.1.1.1")
	})

	t.Run("mismatched mode and network", func(t *testing.T) {
		DNS.Mode = xnet.ModeTLS
		DNS.Network = "udp"
		defer func() { DNS.Network = "tcp" }()

		err := DNS.Validate()
		require.EqualError(t, err, "mismatched mode and network: tls udp")
	})

	t.Run("invalid port", func(t *testing.T) {
		DNS.Port = "foo port"
		defer func() { DNS.Port = "443" }()

		err := DNS.Validate()
		require.Error(t, err)
	})

	t.Run("ok", func(t *testing.T) {
		DNS.Host = "localhost"
		DNS.Mode = xnet.ModeTLS
		DNS.Network = "tcp"
		DNS.Port = "443"

		err := DNS.Validate()
		require.NoError(t, err)
	})

	testsuite.IsDestroyed(t, &DNS)
}

func TestDNS_Marshal(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	DNS := DNS{
		Host:    "localhost",
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Port:    "443",
	}

	t.Run("ok", func(t *testing.T) {
		data, err := DNS.Marshal()
		require.NoError(t, err)

		t.Log(string(data))
	})

	t.Run("failed", func(t *testing.T) {
		DNS.Port = "foo port"

		data, err := DNS.Marshal()
		require.Error(t, err)
		require.Nil(t, data)
	})

	testsuite.IsDestroyed(t, &DNS)
}

func TestDNS_Unmarshal(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	DNS := DNS{}

	t.Run("ok", func(t *testing.T) {
		DNS.Host = "localhost"
		DNS.Mode = xnet.ModeTLS
		DNS.Network = "tcp"
		DNS.Port = "443"

		data, err := DNS.Marshal()
		require.NoError(t, err)

		err = DNS.Unmarshal(data)
		require.NoError(t, err)
	})

	t.Run("invalid config", func(t *testing.T) {
		err := DNS.Unmarshal([]byte{0x00})
		require.Error(t, err)
	})

	t.Run("incorrect config", func(t *testing.T) {
		err := DNS.Unmarshal(nil)
		require.Error(t, err)
	})

	testsuite.IsDestroyed(t, &DNS)
}

func TestDNS_Resolve(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, _, proxyMgr, _ := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	if testsuite.IPv4Enabled {
		t.Run("IPv4", func(t *testing.T) {
			listeners := []*Listener{{
				Mode:    xnet.ModeTLS,
				Network: "tcp",
				Address: "127.0.0.1:443",
			}}

			DNS := new(DNS)
			DNS.Host = "localhost"
			DNS.Mode = xnet.ModeTLS
			DNS.Network = "tcp"
			DNS.Port = "443"
			DNS.Options.Mode = dns.ModeSystem
			DNS.Options.Type = dns.TypeIPv4

			data, err := DNS.Marshal()
			require.NoError(t, err)

			testsuite.IsDestroyed(t, DNS)

			DNS = NewDNS(context.Background(), dnsClient)
			err = DNS.Unmarshal(data)
			require.NoError(t, err)

			t.Run("common", func(t *testing.T) {
				for i := 0; i < 10; i++ {
					resolved, err := DNS.Resolve()
					require.NoError(t, err)
					resolved = testDecryptListeners(resolved)
					require.Equal(t, listeners, resolved)
				}
			})

			t.Run("parallel", func(t *testing.T) {
				testsuite.RunMultiTimes(20, func() {
					resolved, err := DNS.Resolve()
					require.NoError(t, err)
					resolved = testDecryptListeners(resolved)
					require.Equal(t, listeners, resolved)
				})
			})

			testsuite.IsDestroyed(t, DNS)
		})
	}

	if testsuite.IPv6Enabled {
		t.Run("IPv6", func(t *testing.T) {
			listeners := []*Listener{{
				Mode:    xnet.ModeTLS,
				Network: "tcp",
				Address: "[::1]:443",
			}}

			DNS := new(DNS)
			DNS.Host = "localhost"
			DNS.Mode = xnet.ModeTLS
			DNS.Network = "tcp"
			DNS.Port = "443"
			DNS.Options.Mode = dns.ModeSystem
			DNS.Options.Type = dns.TypeIPv6

			data, err := DNS.Marshal()
			require.NoError(t, err)

			testsuite.IsDestroyed(t, DNS)

			DNS = NewDNS(context.Background(), dnsClient)
			err = DNS.Unmarshal(data)
			require.NoError(t, err)

			t.Run("common", func(t *testing.T) {
				for i := 0; i < 10; i++ {
					resolved, err := DNS.Resolve()
					require.NoError(t, err)
					resolved = testDecryptListeners(resolved)
					require.Equal(t, listeners, resolved)
				}
			})

			t.Run("parallel", func(t *testing.T) {
				testsuite.RunMultiTimes(20, func() {
					resolved, err := DNS.Resolve()
					require.NoError(t, err)
					resolved = testDecryptListeners(resolved)
					require.Equal(t, listeners, resolved)
				})
			})

			testsuite.IsDestroyed(t, DNS)
		})
	}

	t.Run("failed", func(t *testing.T) {
		DNS := NewDNS(context.Background(), dnsClient)

		config := []byte(`
         host    = "localhost"
         mode    = "tls"
         network = "tcp"
         port    = "443"
         
         [options]
           mode = "foo mode"`)

		err := DNS.Unmarshal(config)
		require.NoError(t, err)

		t.Run("common", func(t *testing.T) {
			for i := 0; i < 10; i++ {
				listeners, err := DNS.Resolve()
				require.Error(t, err)
				require.Nil(t, listeners)
			}
		})

		t.Run("parallel", func(t *testing.T) {
			testsuite.RunMultiTimes(20, func() {
				listeners, err := DNS.Resolve()
				require.Error(t, err)
				require.Nil(t, listeners)
			})
		})
	})
}

func TestDNSPanic(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("no CBC", func(t *testing.T) {
		DNS := DNS{}

		func() {
			defer testsuite.DeferForPanic(t)
			_, _ = DNS.Resolve()
		}()

		testsuite.IsDestroyed(t, &DNS)
	})

	t.Run("invalid options", func(t *testing.T) {
		DNS := DNS{}

		func() {
			var err error
			key := bytes.Repeat([]byte{0}, aes.Key128Bit)
			DNS.cbc, err = aes.NewCBC(key, key)
			require.NoError(t, err)
			DNS.enc, err = DNS.cbc.Encrypt(testsuite.Bytes())
			require.NoError(t, err)

			defer testsuite.DeferForPanic(t)
			_, _ = DNS.Resolve()
		}()

		testsuite.IsDestroyed(t, &DNS)
	})
}

func TestDNSOptions(t *testing.T) {
	config, err := ioutil.ReadFile("testdata/dns.toml")
	require.NoError(t, err)

	// check unnecessary field
	DNS := DNS{}
	err = toml.Unmarshal(config, &DNS)
	require.NoError(t, err)
	err = DNS.Validate()
	require.NoError(t, err)

	// check zero value
	testsuite.CheckOptions(t, DNS)

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "localhost", actual: DNS.Host},
		{expected: xnet.ModeTLS, actual: DNS.Mode},
		{expected: "tcp", actual: DNS.Network},
		{expected: "443", actual: DNS.Port},
		{expected: dns.ModeSystem, actual: DNS.Options.Mode},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}
