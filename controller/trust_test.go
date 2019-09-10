package controller

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/xnet"
)

func TestTrustNode(t *testing.T) {
	NODE := testGenerateNode(t, true)
	defer NODE.Exit(nil)
	initCtrl(t)
	n := bootstrap.Node{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "localhost:9950",
	}
	req, err := ctrl.TrustNode(&n)
	require.NoError(t, err)
	err = ctrl.ConfirmTrustNode(&n, req)
	require.NoError(t, err)
}
