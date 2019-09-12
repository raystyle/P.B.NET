package controller

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/xnet"
)

func TestNewSClient(t *testing.T) {
	NODE := testGenerateNode(t, true)
	defer NODE.Exit(nil)
	testInitCtrl(t)
	config := &clientCfg{
		Node: &bootstrap.Node{
			Mode:    xnet.TLS,
			Network: "tcp",
			Address: "localhost:9950",
		},
	}
	config.TLSConfig.InsecureSkipVerify = true
	sClient, err := newSClient(ctrl.syncer, config)
	require.NoError(t, err)
	sClient.Close()
}
