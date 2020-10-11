package msfrpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestClient_AuthLogin(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateClient(t)

	t.Run("success", func(t *testing.T) {
		err := msfrpc.AuthLogin()
		require.NoError(t, err)
	})

	t.Run("failed to login", func(t *testing.T) {
		msfrpc.password = "foo"
		err := msfrpc.AuthLogin()
		require.EqualError(t, err, "Login Failed")

		msfrpc.password = testUsername
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.AuthLogin()
			monkey.IsMonkeyError(t, err)
		})
	})

	err := msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}

func TestClient_AuthLogout(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateClient(t)

	t.Run("logout self", func(t *testing.T) {
		err := msfrpc.AuthLogin()
		require.NoError(t, err)

		err = msfrpc.AuthLogout(msfrpc.GetToken())
		require.NoError(t, err)
	})

	t.Run("logout invalid token", func(t *testing.T) {
		err := msfrpc.AuthLogin()
		require.NoError(t, err)

		err = msfrpc.AuthLogout(testInvalidToken)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.AuthLogout(msfrpc.GetToken())
			monkey.IsMonkeyError(t, err)
		})
	})

	err := msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}

func TestClient_AuthTokenList(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		token := msfrpc.GetToken()
		tokens, err := msfrpc.AuthTokenList(ctx)
		require.NoError(t, err)
		var exist bool
		for i := 0; i < len(tokens); i++ {
			t.Log(tokens[i])
			if token == tokens[i] {
				exist = true
			}
		}
		require.True(t, exist)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		tokens, err := msfrpc.AuthTokenList(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, tokens)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			tokens, err := msfrpc.AuthTokenList(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, tokens)
		})
	})

	err := msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}

func TestClient_AuthTokenGenerate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		token, err := msfrpc.AuthTokenGenerate(ctx)
		require.NoError(t, err)
		t.Log(token)

		tokens, err := msfrpc.AuthTokenList(ctx)
		require.NoError(t, err)
		require.Contains(t, tokens, token)

		err = msfrpc.AuthTokenRemove(ctx, token)
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		token, err := msfrpc.AuthTokenGenerate(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Zero(t, token)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			token, err := msfrpc.AuthTokenGenerate(ctx)
			monkey.IsMonkeyError(t, err)
			require.Zero(t, token)
		})
	})

	err := msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}

func TestClient_AuthTokenAdd(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateClientAndLogin(t)
	ctx := context.Background()
	const token = "TEST0123456789012345678901234567"

	t.Run("success", func(t *testing.T) {
		err := msfrpc.AuthTokenAdd(ctx, token)
		require.NoError(t, err)

		tokens, err := msfrpc.AuthTokenList(ctx)
		require.NoError(t, err)
		require.Contains(t, tokens, token)

		err = msfrpc.AuthTokenRemove(ctx, token)
		require.NoError(t, err)
	})

	t.Run("add invalid token", func(t *testing.T) {
		err := msfrpc.AuthTokenAdd(ctx, testInvalidToken)
		require.NoError(t, err)

		tokens, err := msfrpc.AuthTokenList(ctx)
		require.NoError(t, err)
		require.Contains(t, tokens, testInvalidToken)

		err = msfrpc.AuthTokenRemove(ctx, testInvalidToken)
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		// due to the last sub test added testInvalidToken,
		// so must change the token that will be set
		former := msfrpc.GetToken()
		defer msfrpc.SetToken(former)
		msfrpc.SetToken(testInvalidToken + "foo")
		err := msfrpc.AuthTokenAdd(ctx, token)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.AuthTokenAdd(ctx, token)
			monkey.IsMonkeyError(t, err)
		})
	})

	err := msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}

func TestClient_AuthTokenRemove(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateClientAndLogin(t)
	ctx := context.Background()
	const token = "TEST0123456789012345678901234567"

	t.Run("success", func(t *testing.T) {
		err := msfrpc.AuthTokenRemove(ctx, token)
		require.NoError(t, err)

		tokens, err := msfrpc.AuthTokenList(ctx)
		require.NoError(t, err)
		require.NotContains(t, tokens, token)
	})

	t.Run("remove invalid token", func(t *testing.T) {
		err := msfrpc.AuthTokenAdd(ctx, testInvalidToken)
		require.NoError(t, err)

		err = msfrpc.AuthTokenRemove(ctx, testInvalidToken)
		require.NoError(t, err)

		// doesn't exist
		err = msfrpc.AuthTokenRemove(ctx, testInvalidToken)
		require.NoError(t, err)

		tokens, err := msfrpc.AuthTokenList(ctx)
		require.NoError(t, err)
		require.NotContains(t, tokens, testInvalidToken)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		former := msfrpc.GetToken()
		defer msfrpc.SetToken(former)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.AuthTokenRemove(ctx, token)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.AuthTokenRemove(ctx, token)
			monkey.IsMonkeyError(t, err)
		})
	})

	err := msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}
