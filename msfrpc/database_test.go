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

var testDBHost = &DBReportHost{
	Name:         "test-host",
	Host:         "1.2.3.4",
	MAC:          "AA-BB-CC-DD-EE-FF",
	OSName:       "Windows",
	OSFlavor:     "10 Pro",
	OSLanguage:   "zh-cn",
	Architecture: "x64",
	State:        "alive",
	VirtualHost:  "VMWare",
}

func TestMSFRPC_DBReportHost(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)
	})

	t.Run("invalid workspace", func(t *testing.T) {
		testDBHost.Workspace = "foo"
		defer func() { testDBHost.Workspace = "" }()

		err := msfrpc.DBReportHost(ctx, testDBHost)
		require.EqualError(t, err, "invalid workspace: foo")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.DBReportHost(ctx, testDBHost)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.DBReportHost(ctx, testDBHost)
			monkey.IsMonkeyError(t, err)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBHosts(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)

		hosts, err := msfrpc.DBHosts(ctx, "")
		require.NoError(t, err)
		for i := 0; i < len(hosts); i++ {
			t.Log(hosts[i].Name)
			t.Log(hosts[i].Address)
			t.Log(hosts[i].OSName)
		}
	})

	t.Run("invalid workspace", func(t *testing.T) {
		hosts, err := msfrpc.DBHosts(ctx, "foo")
		require.EqualError(t, err, "invalid workspace: foo")
		require.Nil(t, hosts)
	})

	const workspace = "foo"

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		hosts, err := msfrpc.DBHosts(ctx, workspace)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, hosts)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			hosts, err := msfrpc.DBHosts(ctx, workspace)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, hosts)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBGetHost(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)

		hosts, err := msfrpc.DBGetHost(ctx, "", "1.2.3.4")
		require.NoError(t, err)
		require.Len(t, hosts, 1)
		t.Log(hosts[0].Name)
		t.Log(hosts[0].Address)
		t.Log(hosts[0].OSName)
	})

	t.Run("no result", func(t *testing.T) {
		hosts, err := msfrpc.DBGetHost(ctx, "", "9.9.9.9")
		require.NoError(t, err)
		require.Len(t, hosts, 0)
	})

	t.Run("invalid workspace", func(t *testing.T) {
		hosts, err := msfrpc.DBGetHost(ctx, "foo", "1.2.3.4")
		require.EqualError(t, err, "invalid workspace: foo")
		require.Nil(t, hosts)
	})

	const (
		workspace = "foo"
		address   = "9.9.9.9"
	)

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		hosts, err := msfrpc.DBGetHost(ctx, workspace, address)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, hosts)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			hosts, err := msfrpc.DBGetHost(ctx, workspace, address)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, hosts)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
