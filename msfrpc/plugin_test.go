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

func TestClient_PluginLoad(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err := client.PluginLoad(ctx, testPluginFileName, testPluginOptions)
		require.NoError(t, err)
	})

	t.Run("failed", func(t *testing.T) {
		err := client.PluginLoad(ctx, "wmap", testPluginOptions)
		require.EqualError(t, err, "failed to load plugin wmap: failure")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		err := client.PluginLoad(ctx, testPluginFileName, testPluginOptions)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			err := client.PluginLoad(ctx, testPluginFileName, testPluginOptions)
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_PluginUnload(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err := client.PluginLoad(ctx, testPluginFileName, testPluginOptions)
		require.NoError(t, err)

		err = client.PluginUnload(ctx, testPluginName)
		require.NoError(t, err)
	})

	t.Run("failed", func(t *testing.T) {
		err := client.PluginUnload(ctx, "wmap")
		require.EqualError(t, err, "failed to unload plugin wmap: failure")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		err := client.PluginUnload(ctx, testPluginName)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			err := client.PluginUnload(ctx, testPluginName)
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_PluginLoaded(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err := client.PluginLoad(ctx, testPluginFileName, testPluginOptions)
		require.NoError(t, err)

		plugins, err := client.PluginLoaded(ctx)
		require.NoError(t, err)

		require.Contains(t, plugins, testPluginName)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		plugins, err := client.PluginLoaded(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, plugins)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			plugins, err := client.PluginLoaded(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, plugins)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}
