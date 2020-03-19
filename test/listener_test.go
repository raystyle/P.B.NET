package test

import (
	"bytes"
	"context"
	"crypto/tls"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/convert"
	"project/internal/messages"
	"project/internal/option"
	"project/internal/protocol"
	"project/internal/testsuite"
	"project/internal/xnet"

	"project/controller"
	"project/node"
)

func TestNodeListener(t *testing.T) {
	Node := generateInitialNodeAndTrust(t, 0)
	nodeGUID := Node.GUID()

	t.Run("QUIC", func(t *testing.T) {
		testNodeListenerQUIC(t, Node)
	})
	t.Run("Light", func(t *testing.T) {
		testNodeListenerLight(t, Node)
	})
	t.Run("TLS", func(t *testing.T) {
		testNodeListenerTLS(t, Node)
	})

	// clean
	err := ctrl.DeleteNodeUnscoped(nodeGUID)
	require.NoError(t, err)

	Node.Exit(nil)
	testsuite.IsDestroyed(t, Node)
}

func testNodeListenerClientSend(t *testing.T, client *controller.Client) {
	wg := sync.WaitGroup{}
	send := func() {
		defer wg.Done()
		data := bytes.Buffer{}
		for i := 0; i < 1024; i++ {
			data.Write(convert.Int32ToBytes(int32(i)))
			reply, err := client.SendCommand(protocol.TestCommand, data.Bytes())
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

func testNodeListenerQUIC(t *testing.T, node *node.Node) {
	const tag = "l_quic"
	certPEM, keyPEM := generateCert(t).EncodeToPEM()
	listener := messages.Listener{
		Tag:     tag,
		Mode:    xnet.ModeQUIC,
		Network: "udp",
		Address: "localhost:0",
	}
	listener.TLSConfig.Certificates = []option.X509KeyPair{
		{Cert: string(certPEM), Key: string(keyPEM)},
	}
	listener.TLSConfig.LoadFromCertPool.LoadPrivateClientCACerts = true
	listener.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert

	err := node.AddListener(&listener)
	require.NoError(t, err)
	l := getNodeListener(t, node, tag)
	client, err := ctrl.NewClient(context.Background(), l, nil, nil)
	require.NoError(t, err)

	testNodeListenerClientSend(t, client)
}

func testNodeListenerLight(t *testing.T, node *node.Node) {
	const tag = "l_light"
	listener := messages.Listener{
		Tag:     tag,
		Mode:    xnet.ModeLight,
		Network: "tcp",
		Address: "localhost:0",
	}
	err := node.AddListener(&listener)
	require.NoError(t, err)

	l := getNodeListener(t, node, tag)
	client, err := ctrl.NewClient(context.Background(), l, nil, nil)
	require.NoError(t, err)

	testNodeListenerClientSend(t, client)
}

func testNodeListenerTLS(t *testing.T, node *node.Node) {
	const tag = "l_tls"
	certPEM, keyPEM := generateCert(t).EncodeToPEM()
	listener := messages.Listener{
		Tag:     tag,
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "localhost:0",
	}
	listener.TLSConfig.Certificates = []option.X509KeyPair{
		{Cert: string(certPEM), Key: string(keyPEM)},
	}
	listener.TLSConfig.LoadFromCertPool.LoadPrivateClientCACerts = true
	listener.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert

	err := node.AddListener(&listener)
	require.NoError(t, err)
	l := getNodeListener(t, node, tag)
	client, err := ctrl.NewClient(context.Background(), l, nil, nil)
	require.NoError(t, err)

	testNodeListenerClientSend(t, client)
}
