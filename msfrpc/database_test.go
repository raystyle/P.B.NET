package msfrpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

var testDBOptions = &DBConnectOptions{
	Driver:   "postgresql",
	Host:     "127.0.0.1",
	Port:     5433,
	Username: "msf",
	Password: "msf",
	Database: "msf",
	Other:    map[string]interface{}{"foo": "bar"},
}

func TestMSFRPC_DBConnect(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBConnect(ctx, testDBOptions)
		require.NoError(t, err)

		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
	})

	t.Run("failed", func(t *testing.T) {
		driver := testDBOptions.Driver
		testDBOptions.Driver = "foo"
		defer func() { testDBOptions.Driver = driver }()

		err := msfrpc.DBConnect(ctx, testDBOptions)
		require.EqualError(t, err, "failed to connect database")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.DBConnect(ctx, testDBOptions)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.DBConnect(ctx, testDBOptions)
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBDisconnect(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBConnect(ctx, testDBOptions)
		require.NoError(t, err)

		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)

		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.DBDisconnect(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.DBDisconnect(ctx)
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBStatus(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBConnect(ctx, testDBOptions)
		require.NoError(t, err)

		status, err := msfrpc.DBStatus(ctx)
		require.NoError(t, err)
		t.Log("driver:", status.Driver, "database:", status.Database)

		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
	})

	t.Run("null", func(t *testing.T) {
		status, err := msfrpc.DBStatus(ctx)
		require.NoError(t, err)
		t.Log("driver:", status.Driver, "database:", status.Database)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		status, err := msfrpc.DBStatus(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, status)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			status, err := msfrpc.DBStatus(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, status)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
