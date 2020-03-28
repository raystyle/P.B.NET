package msfrpc

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestMSFRPC_CoreModuleStats(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		status, err := msfrpc.CoreModuleStats()
		require.NoError(t, err)
		t.Log("exploit:", status.Exploit)
		t.Log("auxiliary:", status.Auxiliary)
		t.Log("post:", status.Post)
		t.Log("payload:", status.Payload)
		t.Log("encoder:", status.Encoder)
		t.Log("nop:", status.Nop)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		status, err := msfrpc.CoreModuleStats()
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, status)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			status, err := msfrpc.CoreModuleStats()
			monkey.IsMonkeyError(t, err)
			require.Nil(t, status)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_CoreAddModulePath(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		status, err := msfrpc.CoreAddModulePath("")
		require.NoError(t, err)
		t.Log("exploit:", status.Exploit)
		t.Log("auxiliary:", status.Auxiliary)
		t.Log("post:", status.Post)
		t.Log("payload:", status.Payload)
		t.Log("encoder:", status.Encoder)
		t.Log("nop:", status.Nop)
	})

	t.Run("failed", func(t *testing.T) {
		status, err := msfrpc.CoreAddModulePath("foo path")
		require.EqualError(t, err, "The path supplied is not a valid directory.")
		require.Nil(t, status)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		status, err := msfrpc.CoreAddModulePath("")
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, status)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			status, err := msfrpc.CoreAddModulePath("")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, status)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
