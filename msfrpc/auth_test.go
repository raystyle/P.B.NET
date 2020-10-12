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

	client := testGenerateClient(t)

	t.Run("success", func(t *testing.T) {
		err := client.AuthLogin()
		require.NoError(t, err)
	})

	t.Run("failed to login", func(t *testing.T) {
		client.password = "foo"
		err := client.AuthLogin()
		require.EqualError(t, err, "Login Failed")

		client.password = testUsername
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			err := client.AuthLogin()
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_AuthLogout(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClient(t)

	t.Run("logout self", func(t *testing.T) {
		err := client.AuthLogin()
		require.NoError(t, err)

		err = client.AuthLogout(client.GetToken())
		require.NoError(t, err)
	})

	t.Run("logout invalid token", func(t *testing.T) {
		err := client.AuthLogin()
		require.NoError(t, err)

		err = client.AuthLogout(testInvalidToken)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			err := client.AuthLogout(client.GetToken())
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_AuthTokenList(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		token := client.GetToken()
		tokens, err := client.AuthTokenList(ctx)
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
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		tokens, err := client.AuthTokenList(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, tokens)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			tokens, err := client.AuthTokenList(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, tokens)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_AuthTokenGenerate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		token, err := client.AuthTokenGenerate(ctx)
		require.NoError(t, err)
		t.Log(token)

		tokens, err := client.AuthTokenList(ctx)
		require.NoError(t, err)
		require.Contains(t, tokens, token)

		err = client.AuthTokenRemove(ctx, token)
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		token, err := client.AuthTokenGenerate(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Zero(t, token)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			token, err := client.AuthTokenGenerate(ctx)
			monkey.IsMonkeyError(t, err)
			require.Zero(t, token)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_AuthTokenAdd(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()
	const token = "TEST0123456789012345678901234567"

	t.Run("success", func(t *testing.T) {
		err := client.AuthTokenAdd(ctx, token)
		require.NoError(t, err)

		tokens, err := client.AuthTokenList(ctx)
		require.NoError(t, err)
		require.Contains(t, tokens, token)

		err = client.AuthTokenRemove(ctx, token)
		require.NoError(t, err)
	})

	t.Run("add invalid token", func(t *testing.T) {
		err := client.AuthTokenAdd(ctx, testInvalidToken)
		require.NoError(t, err)

		tokens, err := client.AuthTokenList(ctx)
		require.NoError(t, err)
		require.Contains(t, tokens, testInvalidToken)

		err = client.AuthTokenRemove(ctx, testInvalidToken)
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		// due to the last sub test added testInvalidToken,
		// so must change the token that will be set
		former := client.GetToken()
		defer client.SetToken(former)
		client.SetToken(testInvalidToken + "foo")
		err := client.AuthTokenAdd(ctx, token)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			err := client.AuthTokenAdd(ctx, token)
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_AuthTokenRemove(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()
	const token = "TEST0123456789012345678901234567"

	t.Run("success", func(t *testing.T) {
		err := client.AuthTokenRemove(ctx, token)
		require.NoError(t, err)

		tokens, err := client.AuthTokenList(ctx)
		require.NoError(t, err)
		require.NotContains(t, tokens, token)
	})

	t.Run("remove invalid token", func(t *testing.T) {
		err := client.AuthTokenAdd(ctx, testInvalidToken)
		require.NoError(t, err)

		err = client.AuthTokenRemove(ctx, testInvalidToken)
		require.NoError(t, err)

		// doesn't exist
		err = client.AuthTokenRemove(ctx, testInvalidToken)
		require.NoError(t, err)

		tokens, err := client.AuthTokenList(ctx)
		require.NoError(t, err)
		require.NotContains(t, tokens, testInvalidToken)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		former := client.GetToken()
		defer client.SetToken(former)
		client.SetToken(testInvalidToken)

		err := client.AuthTokenRemove(ctx, token)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			err := client.AuthTokenRemove(ctx, token)
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}
