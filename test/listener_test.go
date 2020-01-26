package test

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/crypto/cert"
	"project/internal/messages"
	"project/internal/option"
	"project/internal/protocol"
	"project/internal/testsuite"
	"project/internal/xnet"

	"project/controller"
	"project/node"
)

func TestNodeListener(t *testing.T) {
	NODE := generateNodeAndTrust(t)
	defer NODE.Exit(nil)

	t.Run("parallel", func(t *testing.T) {
		t.Run("QUIC", func(t *testing.T) {
			testNodeListenerQUIC(t, NODE)
		})
		t.Run("Light", func(t *testing.T) {
			testNodeListenerLight(t, NODE)
		})
		t.Run("TLS", func(t *testing.T) {
			testNodeListenerTLS(t, NODE)
		})
	})
}

func testNodeListenerClientSend(t *testing.T, client *controller.Client) {
	wg := sync.WaitGroup{}
	send := func() {
		defer wg.Done()
		data := bytes.Buffer{}
		for i := 0; i < 1024; i++ {
			data.Write(convert.Int32ToBytes(int32(i)))
			reply, err := client.Send(protocol.TestCommand, data.Bytes())
			require.NoError(t, err)
			require.Equal(t, data.Bytes(), reply)
			data.Reset()
		}
	}
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go send()
	}
	wg.Wait()

	client.Close()
	testsuite.IsDestroyed(t, client)
}

func testNodeListenerQUIC(t *testing.T, NODE *node.Node) {
	t.Parallel()

	// generate certificate
	pairs := ctrl.GetSelfCerts()
	opts := cert.Options{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	caCert := pairs[0].Certificate
	caKey := pairs[0].PrivateKey
	pair, err := cert.Generate(caCert, caKey, &opts)
	require.NoError(t, err)

	// generate listener config
	const tag = "l_quic"
	listener := messages.Listener{
		Tag:     tag,
		Mode:    xnet.ModeQUIC,
		Network: "udp",
		Address: "localhost:0",
	}
	c, k := pair.EncodeToPEM()
	listener.TLSConfig.Certificates = []option.X509KeyPair{
		{Cert: string(c), Key: string(k)},
	}
	err = NODE.AddListener(&listener)
	require.NoError(t, err)

	l, err := NODE.GetListener(tag)
	require.NoError(t, err)
	bListener := &bootstrap.Listener{
		Mode:    xnet.ModeQUIC,
		Network: "udp",
		Address: l.Addr().String(),
	}
	client, err := ctrl.NewClient(context.Background(), bListener, nil, nil)
	require.NoError(t, err)

	testNodeListenerClientSend(t, client)
}

func testNodeListenerLight(t *testing.T, NODE *node.Node) {
	t.Parallel()

	const tag = "l_light"
	listener := messages.Listener{
		Tag:     tag,
		Mode:    xnet.ModeLight,
		Network: "tcp",
		Address: "localhost:0",
	}
	err := NODE.AddListener(&listener)
	require.NoError(t, err)

	l, err := NODE.GetListener(tag)
	require.NoError(t, err)
	bListener := &bootstrap.Listener{
		Mode:    xnet.ModeLight,
		Network: "tcp",
		Address: l.Addr().String(),
	}
	client, err := ctrl.NewClient(context.Background(), bListener, nil, nil)
	require.NoError(t, err)

	testNodeListenerClientSend(t, client)
}

func testNodeListenerTLS(t *testing.T, NODE *node.Node) {
	t.Parallel()

	// generate certificate
	pairs := ctrl.GetSelfCerts()
	opts := cert.Options{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	caCert := pairs[0].Certificate
	caKey := pairs[0].PrivateKey
	pair, err := cert.Generate(caCert, caKey, &opts)
	require.NoError(t, err)

	// generate listener config
	const tag = "l_tls"
	listener := messages.Listener{
		Tag:     tag,
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "localhost:0",
	}
	c, k := pair.EncodeToPEM()
	listener.TLSConfig.Certificates = []option.X509KeyPair{
		{Cert: string(c), Key: string(k)},
	}
	err = NODE.AddListener(&listener)
	require.NoError(t, err)

	l, err := NODE.GetListener(tag)
	require.NoError(t, err)
	bListener := &bootstrap.Listener{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: l.Addr().String(),
	}
	client, err := ctrl.NewClient(context.Background(), bListener, nil, nil)
	require.NoError(t, err)

	testNodeListenerClientSend(t, client)
}
