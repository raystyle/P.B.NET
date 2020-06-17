package bootstrap

import (
	"bytes"
	"context"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/testsuite"
	"project/internal/testsuite/testdns"
	"project/internal/xnet"
)

func TestLoad(t *testing.T) {
	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	ctx := context.Background()

	t.Run("load all", func(t *testing.T) {
		for _, testdata := range [...]*struct {
			mode   string
			config string
		}{
			{mode: ModeHTTP, config: "testdata/http.toml"},
			{mode: ModeDNS, config: "testdata/dns.toml"},
			{mode: ModeDirect, config: "testdata/direct.toml"},
		} {
			config, err := ioutil.ReadFile(testdata.config)
			require.NoError(t, err)

			boot, err := Load(ctx, testdata.mode, config, certPool, proxyPool, dnsClient)
			require.NoError(t, err)
			require.NotNil(t, boot)

			testsuite.IsDestroyed(t, boot)
		}
	})

	t.Run("unknown mode", func(t *testing.T) {
		_, err := Load(ctx, "foo mode", nil, certPool, proxyPool, dnsClient)
		require.EqualError(t, err, "unknown mode: foo mode")
	})

	t.Run("invalid config", func(t *testing.T) {
		_, err := Load(ctx, ModeHTTP, nil, certPool, proxyPool, dnsClient)
		require.Error(t, err)
	})
}

// can't use const string, because call security.CoverString().
func testGenerateListener() *Listener {
	return &Listener{
		Mode:    strings.Repeat(xnet.ModeTLS, 1),
		Network: strings.Repeat("tcp", 1),
		Address: strings.Repeat("127.0.0.1:53123", 1),
	}
}

// can't use const string, because call security.CoverString().
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
	// so we must call it again.
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

		defer testsuite.DeferForPanic(t)
		newListener.Decrypt()
	})

	t.Run("invalid dec", func(t *testing.T) {
		enc, err := newListener.cbc.Encrypt(bytes.Repeat([]byte{1}, aes.BlockSize))
		require.NoError(t, err)
		newListener.enc = enc

		defer testsuite.DeferForPanic(t)
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
	mode := strings.Repeat(xnet.ModeTLS, 1)
	network := strings.Repeat("tcp", 1)
	address := strings.Repeat("127.0.0.1:53123", 1)
	listener := NewListener(mode, network, address)

	expected := "tls (tcp 127.0.0.1:53123)"
	require.Equal(t, expected, listener.String())
}
