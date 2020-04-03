package msfrpc

import (
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

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		err := msfrpc.PluginLoad(testPluginFileName, testPluginOptions)
		require.NoError(t, err)
	})

	t.Run("failed", func(t *testing.T) {
		err := msfrpc.PluginLoad("wmap", testPluginOptions)
		require.EqualError(t, err, "failed to load plugin wmap: failure")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		err := msfrpc.PluginLoad(testPluginFileName, testPluginOptions)
		require.EqualError(t, err, testErrInvalidToken)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.PluginLoad(testPluginFileName, testPluginOptions)
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_PluginUnload(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		err := msfrpc.PluginLoad(testPluginFileName, testPluginOptions)
		require.NoError(t, err)

		err = msfrpc.PluginUnload(testPluginName)
		require.NoError(t, err)
	})

	t.Run("failed", func(t *testing.T) {
		err := msfrpc.PluginUnload("wmap")
		require.EqualError(t, err, "failed to unload plugin wmap: failure")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		err := msfrpc.PluginUnload(testPluginName)
		require.EqualError(t, err, testErrInvalidToken)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.PluginUnload(testPluginName)
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_PluginLoaded(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		err := msfrpc.PluginLoad(testPluginFileName, testPluginOptions)
		require.NoError(t, err)

		plugins, err := msfrpc.PluginLoaded()
		require.NoError(t, err)

		require.Contains(t, plugins, testPluginName)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		plugins, err := msfrpc.PluginLoaded()
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, plugins)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			plugins, err := msfrpc.PluginLoaded()
			monkey.IsMonkeyError(t, err)
			require.Nil(t, plugins)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
