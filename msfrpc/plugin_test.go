package msfrpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

const (
	testPluginFileName = "openvas"
	testPluginName     = "OpenVAS"
)

var testPluginOptions = map[string]string{"opt-a": "a", "opt-b": "b"}

func TestMSFRPC_PluginLoad(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err := msfrpc.PluginLoad(ctx, testPluginFileName, testPluginOptions)
		require.NoError(t, err)
	})

	t.Run("failed", func(t *testing.T) {
		err := msfrpc.PluginLoad(ctx, "wmap", testPluginOptions)
		require.EqualError(t, err, "failed to load plugin wmap: failure")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.PluginLoad(ctx, testPluginFileName, testPluginOptions)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.PluginLoad(ctx, testPluginFileName, testPluginOptions)
			monkey.IsMonkeyError(t, err)
		})
	})

	err := msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_PluginUnload(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err := msfrpc.PluginLoad(ctx, testPluginFileName, testPluginOptions)
		require.NoError(t, err)

		err = msfrpc.PluginUnload(ctx, testPluginName)
		require.NoError(t, err)
	})

	t.Run("failed", func(t *testing.T) {
		err := msfrpc.PluginUnload(ctx, "wmap")
		require.EqualError(t, err, "failed to unload plugin wmap: failure")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.PluginUnload(ctx, testPluginName)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.PluginUnload(ctx, testPluginName)
			monkey.IsMonkeyError(t, err)
		})
	})

	err := msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_PluginLoaded(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err := msfrpc.PluginLoad(ctx, testPluginFileName, testPluginOptions)
		require.NoError(t, err)

		plugins, err := msfrpc.PluginLoaded(ctx)
		require.NoError(t, err)

		require.Contains(t, plugins, testPluginName)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		plugins, err := msfrpc.PluginLoaded(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, plugins)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			plugins, err := msfrpc.PluginLoaded(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, plugins)
		})
	})

	err := msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}
