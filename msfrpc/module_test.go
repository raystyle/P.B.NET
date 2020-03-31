package msfrpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestMSFRPC_ModuleExploits(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := msfrpc.ModuleExploits(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		modules, err := msfrpc.ModuleExploits(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, modules)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			modules, err := msfrpc.ModuleExploits(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleAuxiliary(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := msfrpc.ModuleAuxiliary(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		modules, err := msfrpc.ModuleAuxiliary(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, modules)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			modules, err := msfrpc.ModuleAuxiliary(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModulePost(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := msfrpc.ModulePost(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		modules, err := msfrpc.ModulePost(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, modules)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			modules, err := msfrpc.ModulePost(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModulePayloads(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := msfrpc.ModulePayloads(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		modules, err := msfrpc.ModulePayloads(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, modules)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			modules, err := msfrpc.ModulePayloads(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
