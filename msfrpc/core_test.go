package msfrpc

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
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

func TestMSFRPC_CoreReloadModules(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		status, err := msfrpc.CoreReloadModules()
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
		status, err := msfrpc.CoreReloadModules()
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, status)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			status, err := msfrpc.CoreReloadModules()
			monkey.IsMonkeyError(t, err)
			require.Nil(t, status)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_CoreThreadList(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		list, err := msfrpc.CoreThreadList()
		require.NoError(t, err)
		for id, info := range list {
			t.Logf("id: %d\ninfo: %s\n", id, spew.Sdump(info))
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		list, err := msfrpc.CoreThreadList()
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, list)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			status, err := msfrpc.CoreThreadList()
			monkey.IsMonkeyError(t, err)
			require.Nil(t, status)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_CoreThreadKill(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		err := msfrpc.CoreThreadKill(9999)
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		err := msfrpc.CoreThreadKill(0)
		require.EqualError(t, err, testErrInvalidToken)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.CoreThreadKill(0)
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_CoreSetG(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		err := msfrpc.CoreSetG("test", "test value")
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		err := msfrpc.CoreSetG("test", "test value")
		require.EqualError(t, err, testErrInvalidToken)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.CoreSetG("test", "test value")
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_CoreGetG(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		const (
			name  = "test"
			value = "test value"
		)
		err := msfrpc.CoreSetG(name, value)
		require.NoError(t, err)

		val, err := msfrpc.CoreGetG(name)
		require.NoError(t, err)
		require.Equal(t, value, val)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		value, err := msfrpc.CoreGetG("test")
		require.EqualError(t, err, testErrInvalidToken)
		require.Equal(t, "", value)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			value, err := msfrpc.CoreGetG("test")
			monkey.IsMonkeyError(t, err)
			require.Equal(t, "", value)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
