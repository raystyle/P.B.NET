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

func TestMSFRPC_ModuleEncoders(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := msfrpc.ModuleEncoders(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		modules, err := msfrpc.ModuleEncoders(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, modules)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			modules, err := msfrpc.ModuleEncoders(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleNops(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := msfrpc.ModuleNops(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		modules, err := msfrpc.ModuleNops(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, modules)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			modules, err := msfrpc.ModuleNops(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleEvasion(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := msfrpc.ModuleEvasion(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		modules, err := msfrpc.ModuleEvasion(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, modules)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			modules, err := msfrpc.ModuleEvasion(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleModuleInfo(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		info, err := msfrpc.ModuleInfo(ctx, "exploit", "multi/handler")
		require.NoError(t, err)

		t.Log(info.Type)
		t.Log(info.Name)
		t.Log(info.FullName)
		t.Log(info.Rank)
		t.Log(info.DisclosureDate)
		t.Log(info.Description)
		t.Log(info.License)
		t.Log(info.Filepath)
		t.Log(info.Arch)
		t.Log(info.Platform)
		t.Log(info.Authors)
		t.Log(info.Privileged)
		t.Log(info.References)
		t.Log(info.Targets)
		t.Log(info.DefaultTarget)
		t.Log(info.Stance)
		t.Log("options:")
		for key, opt := range info.Options {
			t.Log(key)
			t.Log(opt.Type)
			t.Log(opt.Required)
			t.Log(opt.Advanced)
			t.Log(opt.Description)
			t.Log(opt.Default)
		}
	})

	t.Run("failed", func(t *testing.T) {
		info, err := msfrpc.ModuleInfo(ctx, "foo type", "bar name")
		require.EqualError(t, err, "Invalid Module")
		require.Nil(t, info)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		info, err := msfrpc.ModuleInfo(ctx, "foo", "bar")
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, info)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			info, err := msfrpc.ModuleInfo(ctx, "foo", "bar")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, info)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
