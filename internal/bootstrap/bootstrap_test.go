package bootstrap

import (
	"bytes"
	"context"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/testsuite/testdns"
	"project/internal/xnet"
)

// cannot use const string, because call security.CoverString().
func testGenerateListener() *Listener {
	return &Listener{
		Mode:    strings.Repeat(xnet.ModeTLS, 1),
		Network: strings.Repeat("tcp", 1),
		Address: strings.Repeat("127.0.0.1:53123", 1),
	}
}

func testGenerateListeners() []*Listener {
	listeners := make([]*Listener, 2)
	listeners[0] = &Listener{
		Mode:    strings.Repeat(xnet.ModeTLS, 1),
		Network: strings.Repeat("tcp", 1),
		Address: strings.Repeat("127.0.0.1:53123", 1),
	}
	listeners[1] = &Listener{
		Mode:    strings.Repeat(xnet.ModeTLS, 1),
		Network: strings.Repeat("tcp", 1),
		Address: strings.Repeat("[::1]:53123", 1),
	}
	return listeners
}

func testDecryptListeners(listeners []*Listener) []*Listener {
	l := len(listeners)
	newListeners := make([]*Listener, l)
	for i := 0; i < l; i++ {
		newListeners[i] = listeners[i].Decrypt()
	}
	return newListeners
}

func TestListener(t *testing.T) {
	rawListener := testGenerateListener()
	newListener := NewListener(rawListener.Mode, rawListener.Network, rawListener.Address)
	// rawListener's fields will be covered after call NewListener
	rawListener = testGenerateListener()
	require.NotEqual(t, rawListener.Mode, newListener.Mode)
	require.NotEqual(t, rawListener.Network, newListener.Network)
	require.NotEqual(t, rawListener.Address, newListener.Address)

	t.Run("decrypt", func(t *testing.T) {
		decListener := newListener.Decrypt()
		require.Equal(t, rawListener, decListener)

		decListener.Destroy()
		require.NotEqual(t, rawListener, decListener)
	})

	t.Run("invalid enc", func(t *testing.T) {
		newListener.enc = nil
		defer func() {
			r := recover()
			require.NotNil(t, r)
			t.Log(r)
		}()
		newListener.Decrypt()
	})

	t.Run("invalid dec", func(t *testing.T) {
		enc, err := newListener.cbc.Encrypt(bytes.Repeat([]byte{1}, aes.BlockSize))
		require.NoError(t, err)
		newListener.enc = enc
		defer func() {
			r := recover()
			require.NotNil(t, r)
			t.Log(r)
		}()
		newListener.Decrypt()
	})
}

func TestListener_Equal(t *testing.T) {
	listeners := testGenerateListeners()
	l1 := NewListener(listeners[0].Mode, listeners[0].Network, listeners[0].Address)
	l2 := NewListener(listeners[1].Mode, listeners[1].Network, listeners[1].Address)
	require.False(t, l1.Equal(l2))

	listeners = testGenerateListeners()
	l2 = NewListener(listeners[0].Mode, listeners[0].Network, listeners[0].Address)
	require.True(t, l1.Equal(l2))
}

func TestListener_String(t *testing.T) {
	listener := Listener{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "127.0.0.1:443",
	}
	expect := "tls (tcp 127.0.0.1:443)"
	require.Equal(t, expect, listener.String())
}

func TestLoad(t *testing.T) {
	dnsClient, proxyPool, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()

	ctx := context.Background()

	testdata := [...]*struct {
		mode   string
		config string
	}{
		{mode: ModeHTTP, config: "testdata/http.toml"},
		{mode: ModeDNS, config: "testdata/dns.toml"},
		{mode: ModeDirect, config: "testdata/direct.toml"},
	}
	for _, td := range testdata {
		config, err := ioutil.ReadFile(td.config)
		require.NoError(t, err)
		_, err = Load(ctx, td.mode, config, proxyPool, dnsClient)
		require.NoError(t, err)
	}

	// unknown mode
	_, err := Load(ctx, "foo mode", nil, proxyPool, dnsClient)
	require.EqualError(t, err, "unknown mode: foo mode")

	// invalid config
	_, err = Load(ctx, ModeHTTP, nil, proxyPool, dnsClient)
	require.Error(t, err)
}
