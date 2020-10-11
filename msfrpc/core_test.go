package msfrpc

import (
	"context"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestClient_CoreModuleStats(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		status, err := client.CoreModuleStats(ctx)
		require.NoError(t, err)
		t.Log("exploit:", status.Exploit)
		t.Log("auxiliary:", status.Auxiliary)
		t.Log("post:", status.Post)
		t.Log("payload:", status.Payload)
		t.Log("encoder:", status.Encoder)
		t.Log("nop:", status.Nop)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		status, err := client.CoreModuleStats(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, status)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			status, err := client.CoreModuleStats(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, status)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_CoreAddModulePath(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		status, err := client.CoreAddModulePath(ctx, "")
		require.NoError(t, err)
		t.Log("exploit:", status.Exploit)
		t.Log("auxiliary:", status.Auxiliary)
		t.Log("post:", status.Post)
		t.Log("payload:", status.Payload)
		t.Log("encoder:", status.Encoder)
		t.Log("nop:", status.Nop)
	})

	t.Run("invalid directory", func(t *testing.T) {
		status, err := client.CoreAddModulePath(ctx, "foo path")
		require.EqualError(t, err, "foo path is not a valid directory")
		require.Nil(t, status)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		status, err := client.CoreAddModulePath(ctx, "")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, status)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			status, err := client.CoreAddModulePath(ctx, "")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, status)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_CoreReloadModules(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		status, err := client.CoreReloadModules(ctx)
		require.NoError(t, err)
		t.Log("exploit:", status.Exploit)
		t.Log("auxiliary:", status.Auxiliary)
		t.Log("post:", status.Post)
		t.Log("payload:", status.Payload)
		t.Log("encoder:", status.Encoder)
		t.Log("nop:", status.Nop)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		status, err := client.CoreReloadModules(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, status)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			status, err := client.CoreReloadModules(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, status)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_CoreThreadList(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		threads, err := client.CoreThreadList(ctx)
		require.NoError(t, err)
		for id, info := range threads {
			t.Logf("id: %d\ninfo: %s\n", id, spew.Sdump(info))
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		threads, err := client.CoreThreadList(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, threads)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			threads, err := client.CoreThreadList(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, threads)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_CoreThreadKill(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err := client.CoreThreadKill(ctx, 9999)
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		err := client.CoreThreadKill(ctx, 0)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := client.CoreThreadKill(ctx, 0)
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_CoreSetG(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err := client.CoreSetG(ctx, "test", "test value")
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		err := client.CoreSetG(ctx, "test", "test value")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := client.CoreSetG(ctx, "test", "test value")
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_CoreGetG(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		const (
			name  = "test"
			value = "test value"
		)
		err := client.CoreSetG(ctx, name, value)
		require.NoError(t, err)

		val, err := client.CoreGetG(ctx, name)
		require.NoError(t, err)
		require.Equal(t, value, val)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		value, err := client.CoreGetG(ctx, "test")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Zero(t, value)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			value, err := client.CoreGetG(ctx, "test")
			monkey.IsMonkeyError(t, err)
			require.Zero(t, value)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_CoreUnsetG(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		const (
			name  = "test"
			value = "test value"
		)
		err := client.CoreSetG(ctx, name, "test value")
		require.NoError(t, err)
		val, err := client.CoreGetG(ctx, name)
		require.NoError(t, err)
		require.Equal(t, value, val)

		err = client.CoreUnsetG(ctx, name)
		require.NoError(t, err)
		val, err = client.CoreGetG(ctx, name)
		require.NoError(t, err)
		require.Zero(t, val)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		err := client.CoreUnsetG(ctx, "test")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := client.CoreUnsetG(ctx, "test")
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_CoreSave(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err := client.CoreSave(ctx)
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		err := client.CoreSave(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := client.CoreSave(ctx)
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_CoreVersion(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		version, err := client.CoreVersion(ctx)
		require.NoError(t, err)
		t.Log("version:", version.Version)
		t.Log("ruby:", version.Ruby)
		t.Log("api:", version.API)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		version, err := client.CoreVersion(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, version)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			version, err := client.CoreVersion(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, version)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}
