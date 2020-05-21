package msfrpc

import (
	"context"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestMSFRPC_CoreModuleStats(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		status, err := msfrpc.CoreModuleStats(ctx)
		require.NoError(t, err)
		t.Log("exploit:", status.Exploit)
		t.Log("auxiliary:", status.Auxiliary)
		t.Log("post:", status.Post)
		t.Log("payload:", status.Payload)
		t.Log("encoder:", status.Encoder)
		t.Log("nop:", status.Nop)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		status, err := msfrpc.CoreModuleStats(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, status)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			status, err := msfrpc.CoreModuleStats(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, status)
		})
	})

	msfrpc.Kill()

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_CoreAddModulePath(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		status, err := msfrpc.CoreAddModulePath(ctx, "")
		require.NoError(t, err)
		t.Log("exploit:", status.Exploit)
		t.Log("auxiliary:", status.Auxiliary)
		t.Log("post:", status.Post)
		t.Log("payload:", status.Payload)
		t.Log("encoder:", status.Encoder)
		t.Log("nop:", status.Nop)
	})

	t.Run("invalid directory", func(t *testing.T) {
		status, err := msfrpc.CoreAddModulePath(ctx, "foo path")
		require.EqualError(t, err, "foo path is not a valid directory")
		require.Nil(t, status)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		status, err := msfrpc.CoreAddModulePath(ctx, "")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, status)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			status, err := msfrpc.CoreAddModulePath(ctx, "")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, status)
		})
	})

	msfrpc.Kill()

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_CoreReloadModules(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		status, err := msfrpc.CoreReloadModules(ctx)
		require.NoError(t, err)
		t.Log("exploit:", status.Exploit)
		t.Log("auxiliary:", status.Auxiliary)
		t.Log("post:", status.Post)
		t.Log("payload:", status.Payload)
		t.Log("encoder:", status.Encoder)
		t.Log("nop:", status.Nop)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		status, err := msfrpc.CoreReloadModules(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, status)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			status, err := msfrpc.CoreReloadModules(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, status)
		})
	})

	msfrpc.Kill()

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_CoreThreadList(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		threads, err := msfrpc.CoreThreadList(ctx)
		require.NoError(t, err)
		for id, info := range threads {
			t.Logf("id: %d\ninfo: %s\n", id, spew.Sdump(info))
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		threads, err := msfrpc.CoreThreadList(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, threads)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			threads, err := msfrpc.CoreThreadList(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, threads)
		})
	})

	msfrpc.Kill()

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_CoreThreadKill(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err := msfrpc.CoreThreadKill(ctx, 9999)
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.CoreThreadKill(ctx, 0)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.CoreThreadKill(ctx, 0)
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_CoreSetG(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err := msfrpc.CoreSetG(ctx, "test", "test value")
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.CoreSetG(ctx, "test", "test value")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.CoreSetG(ctx, "test", "test value")
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_CoreGetG(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		const (
			name  = "test"
			value = "test value"
		)
		err := msfrpc.CoreSetG(ctx, name, value)
		require.NoError(t, err)

		val, err := msfrpc.CoreGetG(ctx, name)
		require.NoError(t, err)
		require.Equal(t, value, val)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		value, err := msfrpc.CoreGetG(ctx, "test")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Zero(t, value)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			value, err := msfrpc.CoreGetG(ctx, "test")
			monkey.IsMonkeyError(t, err)
			require.Zero(t, value)
		})
	})

	msfrpc.Kill()

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_CoreUnsetG(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		const (
			name  = "test"
			value = "test value"
		)
		err := msfrpc.CoreSetG(ctx, name, "test value")
		require.NoError(t, err)
		val, err := msfrpc.CoreGetG(ctx, name)
		require.NoError(t, err)
		require.Equal(t, value, val)

		err = msfrpc.CoreUnsetG(ctx, name)
		require.NoError(t, err)
		val, err = msfrpc.CoreGetG(ctx, name)
		require.NoError(t, err)
		require.Zero(t, val)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.CoreUnsetG(ctx, "test")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.CoreUnsetG(ctx, "test")
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_CoreSave(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err := msfrpc.CoreSave(ctx)
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.CoreSave(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.CoreSave(ctx)
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_CoreVersion(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		version, err := msfrpc.CoreVersion(ctx)
		require.NoError(t, err)
		t.Log("version:", version.Version)
		t.Log("ruby:", version.Ruby)
		t.Log("api:", version.API)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		version, err := msfrpc.CoreVersion(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, version)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			version, err := msfrpc.CoreVersion(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, version)
		})
	})

	msfrpc.Kill()

	testsuite.IsDestroyed(t, msfrpc)
}
