package controller

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/info"
	"project/internal/xnet"
)

func TestTrustNodeAndConfirm(t *testing.T) {
	testInitCtrl(t)
	NODE := testGenerateNode(t, true)
	defer NODE.Exit(nil)
	node := &bootstrap.Node{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "localhost:62300",
	}
	req, err := ctrl.TrustNode(node)
	require.NoError(t, err)
	require.Equal(t, info.Host(), req.HostInfo)
	t.Log(req.HostInfo)
	err = ctrl.ConfirmTrustNode(node, req)
	require.NoError(t, err)
}
