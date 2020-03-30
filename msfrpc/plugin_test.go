package msfrpc

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

const testPluginName = "openvas"

var testPluginOptions = map[string]string{"opt-a": "a", "opt-b": "b"}

func TestMSFRPC_PluginLoad(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		err := msfrpc.PluginLoad(testPluginName, testPluginOptions)
		require.NoError(t, err)
	})

	t.Run("failed", func(t *testing.T) {
		err := msfrpc.PluginLoad("wmap", testPluginOptions)
		require.EqualError(t, err, "failed to load plugin wmap: failure")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		err := msfrpc.PluginLoad(testPluginName, testPluginOptions)
		require.EqualError(t, err, testErrInvalidToken)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.PluginLoad(testPluginName, testPluginOptions)
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
