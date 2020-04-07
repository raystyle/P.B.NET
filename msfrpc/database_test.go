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
		require.NotEmpty(t, hosts)
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

		host, err := msfrpc.DBGetHost(ctx, "", "1.2.3.4")
		require.NoError(t, err)
		t.Log(host.Name)
		t.Log(host.Address)
		t.Log(host.OSName)
	})

	t.Run("no result", func(t *testing.T) {
		host, err := msfrpc.DBGetHost(ctx, "", "9.9.9.9")
		require.EqualError(t, err, "host: 9.9.9.9 doesn't exist")
		require.Nil(t, host)
	})

	t.Run("invalid workspace", func(t *testing.T) {
		host, err := msfrpc.DBGetHost(ctx, "foo", "1.2.3.4")
		require.EqualError(t, err, "invalid workspace: foo")
		require.Nil(t, host)
	})

	const (
		workspace = "foo"
		address   = "9.9.9.9"
	)

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		host, err := msfrpc.DBGetHost(ctx, workspace, address)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, host)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			host, err := msfrpc.DBGetHost(ctx, workspace, address)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, host)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBDelHost(t *testing.T) {
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

		hosts, err := msfrpc.DBDelHost(ctx, "", "1.2.3.4")
		require.NoError(t, err)
		require.Len(t, hosts, 1)
	})

	t.Run("empty", func(t *testing.T) {
		hosts, err := msfrpc.DBDelHost(ctx, "", "0.0.0.0")
		require.NoError(t, err)
		require.Len(t, hosts, 0)
	})

	t.Run("invalid workspace", func(t *testing.T) {
		hosts, err := msfrpc.DBDelHost(ctx, "foo", "1.2.3.4")
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

		hosts, err := msfrpc.DBDelHost(ctx, workspace, address)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, hosts)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			hosts, err := msfrpc.DBDelHost(ctx, workspace, address)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, hosts)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

var testDBService = &DBReportService{
	Host:     "1.2.3.4",
	Port:     "445",
	Protocol: "tcp",
	Name:     "smb",
}

func TestMSFRPC_DBReportService(t *testing.T) {
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
		err := msfrpc.DBReportService(ctx, testDBService)
		require.NoError(t, err)
	})

	t.Run("invalid workspace", func(t *testing.T) {
		testDBService.Workspace = "foo"
		defer func() { testDBService.Workspace = "" }()

		err := msfrpc.DBReportService(ctx, testDBService)
		require.EqualError(t, err, "invalid workspace: foo")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.DBReportService(ctx, testDBService)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.DBReportService(ctx, testDBService)
			monkey.IsMonkeyError(t, err)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBServices(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	opts := &DBServicesOptions{
		Limit:    65535,
		Address:  "1.2.3.4",
		Port:     "445",
		Protocol: "tcp",
		Name:     "smb",
	}

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBReportService(ctx, testDBService)
		require.NoError(t, err)

		services, err := msfrpc.DBServices(ctx, opts)
		require.NoError(t, err)
		require.NotEmpty(t, services)
		for i := 0; i < len(services); i++ {
			t.Log(services[i].Host)
			t.Log(services[i].Port)
			t.Log(services[i].Name)
		}
	})

	t.Run("invalid workspace", func(t *testing.T) {
		opts.Workspace = "foo"
		defer func() { opts.Workspace = "" }()

		services, err := msfrpc.DBServices(ctx, opts)
		require.EqualError(t, err, "invalid workspace: foo")
		require.Nil(t, services)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		services, err := msfrpc.DBServices(ctx, opts)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, services)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			services, err := msfrpc.DBServices(ctx, opts)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, services)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBGetService(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	opts := &DBGetServiceOptions{
		Protocol: "tcp",
		Port:     445,
		Names:    "smb",
	}

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBReportService(ctx, testDBService)
		require.NoError(t, err)

		services, err := msfrpc.DBGetService(ctx, opts)
		require.NoError(t, err)
		require.NotEmpty(t, services)
		for i := 0; i < len(services); i++ {
			t.Log(services[i].Host)
			t.Log(services[i].Port)
			t.Log(services[i].Name)
		}
	})

	t.Run("invalid workspace", func(t *testing.T) {
		opts.Workspace = "foo"
		defer func() { opts.Workspace = "" }()

		services, err := msfrpc.DBGetService(ctx, opts)
		require.EqualError(t, err, "invalid workspace: foo")
		require.Nil(t, services)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		services, err := msfrpc.DBGetService(ctx, opts)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, services)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			services, err := msfrpc.DBGetService(ctx, opts)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, services)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBDelService(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	opts := &DBDelServiceOptions{
		Workspace: defaultWorkspace,
		Address:   "1.2.3.4",
		Port:      445,
		Protocol:  "tcp",
	}

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBReportService(ctx, testDBService)
		require.NoError(t, err)

		services, err := msfrpc.DBDelService(ctx, opts)
		require.NoError(t, err)
		require.Len(t, services, 0)
		// TODO [external] msfrpcd bug about DelService
		// file: lib/msf/core/rpc/v10/rpc_db.rb
		// require.Len(t, services, 1)
	})

	t.Run("empty", func(t *testing.T) {
		opts.Address = "9.9.9.9"
		defer func() { opts.Address = "1.2.3.4" }()

		services, err := msfrpc.DBDelService(ctx, opts)
		require.NoError(t, err)
		require.Len(t, services, 0)
	})

	t.Run("invalid workspace", func(t *testing.T) {
		opts.Workspace = "foo"
		defer func() { opts.Workspace = "" }()

		services, err := msfrpc.DBDelService(ctx, opts)
		require.EqualError(t, err, "invalid workspace: foo")
		require.Nil(t, services)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		services, err := msfrpc.DBDelService(ctx, opts)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, services)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			services, err := msfrpc.DBDelService(ctx, opts)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, services)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
