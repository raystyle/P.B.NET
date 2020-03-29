package msfrpc

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestMSFRPC_ConsoleCreate(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		result, err := msfrpc.ConsoleCreate()
		require.NoError(t, err)
		t.Log("id:", result.ID)
		t.Log("prompt:", result.Prompt)
		t.Log("busy:", result.Busy)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		result, err := msfrpc.ConsoleCreate()
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, result)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			result, err := msfrpc.ConsoleCreate()
			monkey.IsMonkeyError(t, err)
			require.Nil(t, result)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ConsoleDestroy(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		result, err := msfrpc.ConsoleCreate()
		require.NoError(t, err)

		err = msfrpc.ConsoleDestroy(result.ID)
		require.NoError(t, err)
	})

	t.Run("invalid console id", func(t *testing.T) {
		err = msfrpc.ConsoleDestroy("999")
		require.EqualError(t, err, "invalid console id: 999")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		err := msfrpc.ConsoleDestroy("foo")
		require.EqualError(t, err, testErrInvalidToken)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.ConsoleDestroy("foo")
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ConsoleList(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		consoles, err := msfrpc.ConsoleList()
		require.NoError(t, err)
		for _, console := range consoles {
			t.Log("id:", console.ID)
			t.Log("prompt:", console.Prompt)
			t.Log("prompt(byte):", []byte(console.Prompt))
			t.Log("busy:", console.Busy)
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		consoles, err := msfrpc.ConsoleList()
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, consoles)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			consoles, err := msfrpc.ConsoleList()
			monkey.IsMonkeyError(t, err)
			require.Nil(t, consoles)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
