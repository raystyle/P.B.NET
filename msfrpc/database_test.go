package msfrpc

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

var testDBOptions = &DBConnectOptions{
	Driver:   "postgresql",
	Host:     "127.0.0.1",
	Port:     5433,
	Username: "msf",
	Password: "msf",
	Database: "msftest",
	Options:  map[string]interface{}{"foo": "bar"},
}

func TestMSFRPC_DBConnect(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)

		err := msfrpc.DBConnect(ctx, testDBOptions)
		require.NoError(t, err)

		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
	})

	t.Run("failed", func(t *testing.T) {
		port := testDBOptions.Port
		testDBOptions.Port = 9999
		defer func() { testDBOptions.Port = port }()

		err := msfrpc.DBConnect(ctx, testDBOptions)
		require.Error(t, err)
	})

	t.Run("invalid driver", func(t *testing.T) {
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

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
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

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
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

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
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
		require.EqualError(t, err, "workspace foo doesn't exist")
	})

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		err := msfrpc.DBReportHost(ctx, testDBHost)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
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

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
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
		require.EqualError(t, err, "workspace foo doesn't exist")
		require.Nil(t, hosts)
	})

	const workspace = "foo"

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		hosts, err := msfrpc.DBHosts(ctx, workspace)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
		require.Nil(t, hosts)
	})

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

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)

		opts := DBGetHostOptions{
			Address: "1.2.3.4",
		}
		host, err := msfrpc.DBGetHost(ctx, &opts)
		require.NoError(t, err)
		t.Log(host.Name)
		t.Log(host.Address)
		t.Log(host.OSName)
	})

	t.Run("no result", func(t *testing.T) {
		opts := DBGetHostOptions{
			Address: "9.9.9.9",
		}
		host, err := msfrpc.DBGetHost(ctx, &opts)
		require.EqualError(t, err, "host: 9.9.9.9 doesn't exist")
		require.Nil(t, host)
	})

	t.Run("invalid workspace", func(t *testing.T) {
		opts := DBGetHostOptions{
			Workspace: "foo",
			Address:   "1.2.3.4",
		}
		host, err := msfrpc.DBGetHost(ctx, &opts)
		require.EqualError(t, err, "workspace foo doesn't exist")
		require.Nil(t, host)
	})

	opts := &DBGetHostOptions{
		Workspace: "foo",
		Address:   "1.2.3.4",
	}

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		host, err := msfrpc.DBGetHost(ctx, opts)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
		require.Nil(t, host)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		host, err := msfrpc.DBGetHost(ctx, opts)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, host)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			host, err := msfrpc.DBGetHost(ctx, opts)
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

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)

		opts := DBDelHostOptions{
			Address: "1.2.3.4",
		}
		hosts, err := msfrpc.DBDelHost(ctx, &opts)
		require.NoError(t, err)
		require.Len(t, hosts, 1)
	})

	t.Run("empty address", func(t *testing.T) {
		hosts, err := msfrpc.DBDelHost(ctx, new(DBDelHostOptions))
		require.NoError(t, err)
		require.Len(t, hosts, 0)
	})

	t.Run("invalid address", func(t *testing.T) {
		opts := DBDelHostOptions{
			Address: "3.3.3.3",
		}
		hosts, err := msfrpc.DBDelHost(ctx, &opts)
		require.EqualError(t, err, "host: 3.3.3.3 doesn't exist in workspace: default")
		require.Nil(t, hosts)
	})

	t.Run("invalid workspace", func(t *testing.T) {
		opts := DBDelHostOptions{
			Workspace: "foo",
		}
		hosts, err := msfrpc.DBDelHost(ctx, &opts)
		require.EqualError(t, err, "workspace foo doesn't exist")
		require.Nil(t, hosts)
	})

	opts := &DBDelHostOptions{
		Workspace: "foo",
		Address:   "1.2.3.4",
	}

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		hosts, err := msfrpc.DBDelHost(ctx, opts)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
		require.Nil(t, hosts)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		hosts, err := msfrpc.DBDelHost(ctx, opts)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, hosts)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			hosts, err := msfrpc.DBDelHost(ctx, opts)
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

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
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
		require.EqualError(t, err, "workspace foo doesn't exist")
	})

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		err := msfrpc.DBReportService(ctx, testDBService)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
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

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
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
		require.EqualError(t, err, "workspace foo doesn't exist")
		require.Nil(t, services)
	})

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		services, err := msfrpc.DBServices(ctx, opts)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
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

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
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
		require.EqualError(t, err, "workspace foo doesn't exist")
		require.Nil(t, services)
	})

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		services, err := msfrpc.DBGetService(ctx, opts)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
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

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	// TODO [external] msfrpcd bug about DelService
	// file: lib/msf/core/rpc/v10/rpc_db.rb
	// only use address
	opts := &DBDelServiceOptions{
		Address: "1.2.3.4",
		// Port:     445,
		// Protocol: "tcp",
	}

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBReportService(ctx, testDBService)
		require.NoError(t, err)

		services, err := msfrpc.DBDelService(ctx, opts)
		require.NoError(t, err)
		require.Len(t, services, 1)
	})

	t.Run("empty address", func(t *testing.T) {
		opts.Address = ""
		defer func() { opts.Address = "1.2.3.4" }()

		services, err := msfrpc.DBDelService(ctx, opts)
		require.NoError(t, err)
		require.Len(t, services, 0)
	})

	t.Run("invalid address", func(t *testing.T) {
		opts.Address = "9.9.9.9"
		defer func() { opts.Address = "1.2.3.4" }()

		services, err := msfrpc.DBDelService(ctx, opts)
		require.EqualError(t, err, "failed to delete service")
		require.Nil(t, services)
	})

	t.Run("invalid workspace", func(t *testing.T) {
		opts.Workspace = "foo"
		defer func() { opts.Workspace = "" }()

		services, err := msfrpc.DBDelService(ctx, opts)
		require.EqualError(t, err, "workspace foo doesn't exist")
		require.Nil(t, services)
	})

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		services, err := msfrpc.DBDelService(ctx, opts)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
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

var testDBClient = &DBReportClient{
	Host:      "1.2.3.4",
	UAString:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:78.0) Gecko/20100101 Firefox/78.0",
	UAName:    "Mozilla",
	UAVersion: "5.0",
}

func TestMSFRPC_DBReportClient(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBReportClient(ctx, testDBClient)
		require.NoError(t, err)
	})

	t.Run("invalid workspace", func(t *testing.T) {
		testDBClient.Workspace = "foo"
		defer func() { testDBClient.Workspace = "" }()

		err := msfrpc.DBReportClient(ctx, testDBClient)
		require.EqualError(t, err, "workspace foo doesn't exist")
	})

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		err := msfrpc.DBReportClient(ctx, testDBClient)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.DBReportClient(ctx, testDBClient)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.DBReportClient(ctx, testDBClient)
			monkey.IsMonkeyError(t, err)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBClients(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	opts := &DBClientsOptions{
		UAName:    "Mozilla",
		UAVersion: "5.0",
	}

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBReportClient(ctx, testDBClient)
		require.NoError(t, err)

		clients, err := msfrpc.DBClients(ctx, opts)
		require.NoError(t, err)
		require.NotEmpty(t, clients)
		for i := 0; i < len(clients); i++ {
			t.Log(clients[i].Host)
			t.Log(clients[i].UAString)
		}
	})

	t.Run("invalid workspace", func(t *testing.T) {
		opts.Workspace = "foo"
		defer func() { opts.Workspace = "" }()

		clients, err := msfrpc.DBClients(ctx, opts)
		require.EqualError(t, err, "workspace foo doesn't exist")
		require.Nil(t, clients)
	})

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		clients, err := msfrpc.DBClients(ctx, opts)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
		require.Nil(t, clients)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		clients, err := msfrpc.DBClients(ctx, opts)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, clients)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			clients, err := msfrpc.DBClients(ctx, opts)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, clients)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBGetClient(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	opts := &DBGetClientOptions{
		Host:     testDBClient.Host,
		UAString: testDBClient.UAString,
	}

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBReportClient(ctx, testDBClient)
		require.NoError(t, err)

		client, err := msfrpc.DBGetClient(ctx, opts)
		require.NoError(t, err)
		t.Log(client.Host)
		t.Log(client.UAString)
	})

	t.Run("no result", func(t *testing.T) {
		opts := DBGetClientOptions{
			Host:     "9.9.9.9",
			UAString: testDBClient.UAString,
		}
		client, err := msfrpc.DBGetClient(ctx, &opts)
		require.EqualError(t, err, "client: 9.9.9.9 doesn't exist")
		require.Nil(t, client)
	})

	t.Run("invalid workspace", func(t *testing.T) {
		opts.Workspace = "foo"
		defer func() { opts.Workspace = "" }()

		client, err := msfrpc.DBGetClient(ctx, opts)
		require.EqualError(t, err, "workspace foo doesn't exist")
		require.Nil(t, client)
	})

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		client, err := msfrpc.DBGetClient(ctx, opts)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
		require.Nil(t, client)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		client, err := msfrpc.DBGetClient(ctx, opts)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, client)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			client, err := msfrpc.DBGetClient(ctx, opts)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, client)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBDelClient(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	opts := &DBDelClientOptions{
		Address:   "1.2.3.4",
		UAName:    testDBClient.UAName,
		UAVersion: testDBClient.UAVersion,
	}

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBReportClient(ctx, testDBClient)
		require.NoError(t, err)

		clients, err := msfrpc.DBDelClient(ctx, opts)
		require.NoError(t, err)
		require.Len(t, clients, 0)

		// TODO [external] msfrpcd bug about DelClient
		// file: lib/msf/core/rpc/v10/rpc_db.rb
		// require.Len(t, clients, 1)
	})

	t.Run("empty address", func(t *testing.T) {
		err := msfrpc.DBReportClient(ctx, testDBClient)
		require.NoError(t, err)

		clients, err := msfrpc.DBDelClient(ctx, new(DBDelClientOptions))
		require.NoError(t, err)
		require.Len(t, clients, 0)
	})

	t.Run("invalid address", func(t *testing.T) {
		opts.Address = "9.9.9.9"
		defer func() { opts.Address = "1.2.3.4" }()

		clients, err := msfrpc.DBDelClient(ctx, opts)
		require.NoError(t, err)
		require.Len(t, clients, 0)
	})

	t.Run("invalid workspace", func(t *testing.T) {
		opts.Workspace = "foo"
		defer func() { opts.Workspace = "" }()

		clients, err := msfrpc.DBDelClient(ctx, opts)
		require.EqualError(t, err, "workspace foo doesn't exist")
		require.Nil(t, clients)
	})

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		clients, err := msfrpc.DBDelClient(ctx, opts)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
		require.Nil(t, clients)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		clients, err := msfrpc.DBDelClient(ctx, opts)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, clients)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			clients, err := msfrpc.DBDelClient(ctx, opts)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, clients)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

var testDBCred = &DBCreateCredentialOptions{
	OriginType:     "service",
	ServiceName:    "smb",
	Address:        "127.0.0.1",
	Port:           445,
	Protocol:       "tcp",
	ModuleFullname: "auxiliary/scanner/smb/smb_login",
	Username:       "Administrator",
	PrivateType:    "password",
	PrivateData:    "pwd",
	WorkspaceID:    1,
}

func TestMSFRPC_DBCreateCredential(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		result, err := msfrpc.DBCreateCredential(ctx, testDBCred)
		require.NoError(t, err)
		t.Log(result.Host)
		t.Log(result.Username)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		result, err := msfrpc.DBCreateCredential(ctx, testDBCred)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, result)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			result, err := msfrpc.DBCreateCredential(ctx, testDBCred)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, result)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBCreds(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		result, err := msfrpc.DBCreateCredential(ctx, testDBCred)
		require.NoError(t, err)
		require.NotNil(t, result)

		creds, err := msfrpc.DBCreds(ctx, "")
		require.NoError(t, err)
		require.NotEmpty(t, creds)
		for i := 0; i < len(creds); i++ {
			t.Log(creds[i].Host)
			t.Log(creds[i].Username)
			t.Log(creds[i].Password)
		}
	})

	t.Run("invalid workspace", func(t *testing.T) {
		creds, err := msfrpc.DBCreds(ctx, "foo")
		require.EqualError(t, err, "workspace foo doesn't exist")
		require.Nil(t, creds)
	})

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		creds, err := msfrpc.DBCreds(ctx, "")
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
		require.Nil(t, creds)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		creds, err := msfrpc.DBCreds(ctx, "")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, creds)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			creds, err := msfrpc.DBCreds(ctx, "")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, creds)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBDelCreds(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		result, err := msfrpc.DBCreateCredential(ctx, testDBCred)
		require.NoError(t, err)
		require.NotNil(t, result)

		creds, err := msfrpc.DBDelCreds(ctx, "")
		require.NoError(t, err)
		require.Len(t, creds, 1)

	})

	t.Run("invalid workspace", func(t *testing.T) {
		creds, err := msfrpc.DBDelCreds(ctx, "foo")
		require.EqualError(t, err, "workspace foo doesn't exist")
		require.Nil(t, creds)
	})

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		creds, err := msfrpc.DBDelCreds(ctx, "")
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
		require.Nil(t, creds)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		creds, err := msfrpc.DBDelCreds(ctx, "")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, creds)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			creds, err := msfrpc.DBDelCreds(ctx, "")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, creds)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

var testDBLoot = &DBReportLoot{
	Host:        "1.9.9.9",
	Name:        "screenshot",
	Type:        "screenshot",
	Path:        "test path",
	Information: "test information",
}

func TestMSFRPC_DBReportLoot(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBReportLoot(ctx, testDBLoot)
		require.NoError(t, err)
	})

	t.Run("invalid workspace", func(t *testing.T) {
		testDBLoot.Workspace = "foo"
		defer func() { testDBLoot.Workspace = "" }()

		err := msfrpc.DBReportLoot(ctx, testDBLoot)
		require.EqualError(t, err, "workspace foo doesn't exist")
	})

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		err := msfrpc.DBReportLoot(ctx, testDBLoot)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.DBReportLoot(ctx, testDBLoot)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.DBReportLoot(ctx, testDBLoot)
			monkey.IsMonkeyError(t, err)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBLoots(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	opts := &DBLootsOptions{
		Limit: 65535,
	}

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBReportLoot(ctx, testDBLoot)
		require.NoError(t, err)

		loots, err := msfrpc.DBLoots(ctx, opts)
		require.NoError(t, err)
		require.NotEmpty(t, loots)
		for i := 0; i < len(loots); i++ {
			t.Log(loots[i].Host)
			t.Log(loots[i].Name)
			t.Log(loots[i].LootType)
		}
	})

	t.Run("invalid workspace", func(t *testing.T) {
		opts.Workspace = "foo"
		defer func() { opts.Workspace = "" }()

		loots, err := msfrpc.DBLoots(ctx, opts)
		require.EqualError(t, err, "workspace foo doesn't exist")
		require.Nil(t, loots)
	})

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		loots, err := msfrpc.DBLoots(ctx, opts)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
		require.Nil(t, loots)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		loots, err := msfrpc.DBLoots(ctx, opts)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, loots)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			loots, err := msfrpc.DBLoots(ctx, opts)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, loots)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBWorkspaces(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		workspaces, err := msfrpc.DBWorkspaces(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, workspaces)
		for i := 0; i < len(workspaces); i++ {
			t.Log(workspaces[i].ID)
			t.Log(workspaces[i].Name)
		}
	})

	t.Run("database not loaded", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		workspaces, err := msfrpc.DBWorkspaces(ctx)
		require.EqualError(t, err, ErrDBNotLoadedFriendly)
		require.Nil(t, workspaces)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		workspaces, err := msfrpc.DBWorkspaces(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, workspaces)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			workspaces, err := msfrpc.DBWorkspaces(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, workspaces)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBGetWorkspace(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		workspace, err := msfrpc.DBGetWorkspace(ctx, defaultWorkspace)
		require.NoError(t, err)
		t.Log(workspace.ID)
		t.Log(workspace.Name)
	})

	t.Run("empty name", func(t *testing.T) {
		workspace, err := msfrpc.DBGetWorkspace(ctx, "")
		require.NoError(t, err)
		require.Equal(t, defaultWorkspace, workspace.Name)
		t.Log(workspace.ID)
		t.Log(workspace.Name)
	})

	t.Run("invalid workspace name", func(t *testing.T) {
		workspace, err := msfrpc.DBGetWorkspace(ctx, "foo")
		require.EqualError(t, err, "workspace foo doesn't exist")
		require.Nil(t, workspace)
	})

	t.Run("database not loaded", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		workspace, err := msfrpc.DBGetWorkspace(ctx, defaultWorkspace)
		require.EqualError(t, err, ErrDBNotLoadedFriendly)
		require.Nil(t, workspace)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		workspace, err := msfrpc.DBGetWorkspace(ctx, defaultWorkspace)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, workspace)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			workspace, err := msfrpc.DBGetWorkspace(ctx, defaultWorkspace)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, workspace)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBAddWorkspace(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const name = "test_add"

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBAddWorkspace(ctx, name)
		require.NoError(t, err)

		workspace, err := msfrpc.DBGetWorkspace(ctx, name)
		require.NoError(t, err)
		require.Equal(t, name, workspace.Name)

		err = msfrpc.DBDelWorkspace(ctx, name)
		require.NoError(t, err)
	})

	t.Run("add twice", func(t *testing.T) {
		err := msfrpc.DBAddWorkspace(ctx, name)
		require.NoError(t, err)
		err = msfrpc.DBAddWorkspace(ctx, name)
		require.NoError(t, err)

		err = msfrpc.DBDelWorkspace(ctx, name)
		require.NoError(t, err)
	})

	t.Run("empty name", func(t *testing.T) {
		workspaces, err := msfrpc.DBWorkspaces(ctx)
		require.NoError(t, err)
		l1 := len(workspaces)

		err = msfrpc.DBAddWorkspace(ctx, "")
		require.NoError(t, err)

		workspaces, err = msfrpc.DBWorkspaces(ctx)
		require.NoError(t, err)
		l2 := len(workspaces)
		require.Equal(t, l1, l2)
	})

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		err := msfrpc.DBAddWorkspace(ctx, name)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.DBAddWorkspace(ctx, name)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.DBAddWorkspace(ctx, name)
			monkey.IsMonkeyError(t, err)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBDelWorkspace(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const name = "test_add"

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBAddWorkspace(ctx, name)
		require.NoError(t, err)

		err = msfrpc.DBDelWorkspace(ctx, name)
		require.NoError(t, err)

		workspace, err := msfrpc.DBGetWorkspace(ctx, name)
		require.Error(t, err)
		require.Nil(t, workspace)
	})

	t.Run("empty name", func(t *testing.T) {
		workspaces, err := msfrpc.DBWorkspaces(ctx)
		require.NoError(t, err)
		l1 := len(workspaces)

		err = msfrpc.DBDelWorkspace(ctx, "")
		require.NoError(t, err)

		workspaces, err = msfrpc.DBWorkspaces(ctx)
		require.NoError(t, err)
		l2 := len(workspaces)
		require.Equal(t, l1, l2)
	})

	t.Run("invalid workspace", func(t *testing.T) {
		err = msfrpc.DBDelWorkspace(ctx, "foo")
		require.EqualError(t, err, "workspace foo doesn't exist")
	})

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		err := msfrpc.DBDelWorkspace(ctx, name)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.DBDelWorkspace(ctx, name)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.DBDelWorkspace(ctx, name)
			monkey.IsMonkeyError(t, err)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBSetWorkspace(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const name = "test_add"

	t.Run("success", func(t *testing.T) {
		err := msfrpc.DBAddWorkspace(ctx, name)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBDelWorkspace(ctx, name)
			require.NoError(t, err)
		}()

		err = msfrpc.DBSetWorkspace(ctx, name)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBSetWorkspace(ctx, defaultWorkspace)
			require.NoError(t, err)
		}()

		workspace, err := msfrpc.DBCurrentWorkspace(ctx)
		require.NoError(t, err)
		require.Equal(t, name, workspace.Name)
	})

	t.Run("empty name", func(t *testing.T) {
		err := msfrpc.DBSetWorkspace(ctx, "")
		require.NoError(t, err)

		workspace, err := msfrpc.DBCurrentWorkspace(ctx)
		require.NoError(t, err)
		require.Equal(t, defaultWorkspace, workspace.Name)
	})

	t.Run("invalid workspace", func(t *testing.T) {
		err = msfrpc.DBSetWorkspace(ctx, "foo")
		require.EqualError(t, err, "workspace foo doesn't exist")
	})

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		err := msfrpc.DBSetWorkspace(ctx, name)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.DBSetWorkspace(ctx, name)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.DBSetWorkspace(ctx, name)
			monkey.IsMonkeyError(t, err)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBCurrentWorkspace(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const name = "test_add"

	t.Run("success", func(t *testing.T) {
		workspace, err := msfrpc.DBCurrentWorkspace(ctx)
		require.NoError(t, err)
		require.Equal(t, defaultWorkspace, workspace.Name)
		t.Log("current workspace id:", workspace.ID)

		err = msfrpc.DBAddWorkspace(ctx, name)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBDelWorkspace(ctx, name)
			require.NoError(t, err)
		}()

		err = msfrpc.DBSetWorkspace(ctx, name)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBSetWorkspace(ctx, defaultWorkspace)
			require.NoError(t, err)
		}()

		workspace, err = msfrpc.DBCurrentWorkspace(ctx)
		require.NoError(t, err)
		require.Equal(t, name, workspace.Name)
	})

	t.Run("empty name", func(t *testing.T) {
		err := msfrpc.DBSetWorkspace(ctx, "")
		require.NoError(t, err)

		workspace, err := msfrpc.DBCurrentWorkspace(ctx)
		require.NoError(t, err)
		require.Equal(t, defaultWorkspace, workspace.Name)
	})

	t.Run("database not loaded", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		workspace, err := msfrpc.DBCurrentWorkspace(ctx)
		require.EqualError(t, err, ErrDBNotLoadedFriendly)
		require.Nil(t, workspace)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		workspace, err := msfrpc.DBCurrentWorkspace(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, workspace)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			workspace, err := msfrpc.DBCurrentWorkspace(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, workspace)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBEvent(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		id := testCreateMeterpreterSession(t, msfrpc, "55200")
		defer func() {
			err = msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		opts := DBEventOptions{
			Limit: 65535,
		}
		events, err := msfrpc.DBEvent(ctx, &opts)
		require.NoError(t, err)
		require.NotEmpty(t, events)
		for i := 0; i < len(events); i++ {
			t.Log("name:", events[i].Name)
			t.Log("host:", events[i].Host)
			t.Log("username:", events[i].Username)
			t.Log("---------information---------")
			for key, value := range events[i].Information {
				if key != "datastore" {
					t.Log(key, value)
				} else {
					t.Log("---------data store----------")
					dataStore := value.(map[string]interface{})
					for key, value := range dataStore {
						t.Log(key, value)
					}
				}
			}
			t.Log("--------------------------------")
		}
	})

	t.Run("invalid workspace", func(t *testing.T) {
		opts := DBEventOptions{
			Workspace: "foo",
		}
		events, err := msfrpc.DBEvent(ctx, &opts)
		require.EqualError(t, err, "workspace foo doesn't exist")
		require.Nil(t, events)
	})

	opts := &DBEventOptions{
		Workspace: "foo",
		Limit:     65535,
	}

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		events, err := msfrpc.DBEvent(ctx, opts)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
		require.Nil(t, events)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		events, err := msfrpc.DBEvent(ctx, opts)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, events)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			events, err := msfrpc.DBEvent(ctx, opts)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, events)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_DBImportData(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		result, err := ioutil.ReadFile("testdata/nmap.xml")
		require.NoError(t, err)

		opts := DBImportDataOptions{
			Data: string(result),
		}
		err = msfrpc.DBImportData(ctx, &opts)
		require.NoError(t, err)

		hosts, err := msfrpc.DBHosts(ctx, "")
		require.NoError(t, err)
		var added bool
		for i := 0; i < len(hosts); i++ {
			if hosts[i].Address == "1.1.1.1" {
				added = true
			}
		}
		require.True(t, added)
	})

	t.Run("no data", func(t *testing.T) {
		err = msfrpc.DBImportData(ctx, new(DBImportDataOptions))
		require.EqualError(t, err, "no data")
	})

	t.Run("invalid workspace", func(t *testing.T) {
		opts := DBImportDataOptions{
			Workspace: "foo",
			Data:      "foo data",
		}
		err = msfrpc.DBImportData(ctx, &opts)
		require.EqualError(t, err, "workspace foo doesn't exist")
	})

	t.Run("invalid data", func(t *testing.T) {
		opts := DBImportDataOptions{
			Data: "foo data",
		}
		err = msfrpc.DBImportData(ctx, &opts)
		require.EqualError(t, err, "invalid file format")
	})

	opts := &DBImportDataOptions{
		Workspace: "foo",
		Data:      "foo data",
	}

	t.Run("database active record", func(t *testing.T) {
		err = msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBConnect(ctx, testDBOptions)
			require.NoError(t, err)
		}()

		err = msfrpc.DBImportData(ctx, opts)
		require.EqualError(t, err, ErrDBActiveRecordFriendly)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err = msfrpc.DBImportData(ctx, opts)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err = msfrpc.DBImportData(ctx, opts)
			monkey.IsMonkeyError(t, err)
		})
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
