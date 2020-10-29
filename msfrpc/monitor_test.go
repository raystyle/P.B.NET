package msfrpc

import (
	"context"
	"io/ioutil"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/patch/monkey"
	"project/internal/patch/toml"
	"project/internal/testsuite"
)

var (
	testBasicMonitorOpts = &MonitorOptions{
		Interval: 25 * time.Millisecond,
	}
	testDBMonitorOpts = &MonitorOptions{
		Interval: 25 * time.Millisecond,
		EnableDB: true,
		Database: testDBOptions,
	}
)

func TestNewMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)

	monitor := NewMonitor(client, nil, nil)

	monitor.Close()

	testsuite.IsDestroyed(t, monitor)

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestMonitor_tokenMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)

	const token = "TEST0123456789012345678901234567"
	ctx := context.Background()

	t.Run("add", func(t *testing.T) {
		var (
			sToken string
			sAdd   bool
		)

		callbacks := MonitorCallbacks{OnToken: func(token string, add bool) {
			sToken = token
			sAdd = add
		}}
		monitor := NewMonitor(client, &callbacks, testBasicMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		err := client.AuthTokenAdd(ctx, token)
		require.NoError(t, err)
		defer func() {
			err = client.AuthTokenRemove(ctx, token)
			require.NoError(t, err)
		}()

		// wait watch
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)

		// compare result
		require.Equal(t, token, sToken)
		require.True(t, sAdd)
	})

	t.Run("delete", func(t *testing.T) {
		err := client.AuthTokenAdd(ctx, token)
		require.NoError(t, err)
		defer func() {
			err = client.AuthTokenRemove(ctx, token)
			require.NoError(t, err)
		}()

		var sToken string
		sAdd := true

		callbacks := MonitorCallbacks{OnToken: func(token string, add bool) {
			sToken = token
			sAdd = add
		}}
		monitor := NewMonitor(client, &callbacks, testBasicMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// wait watch for delete token
		err = client.AuthTokenRemove(ctx, token)
		require.NoError(t, err)

		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)

		// compare result
		require.Equal(t, token, sToken)
		require.False(t, sAdd)
	})

	t.Run("failed to watch", func(t *testing.T) {
		monitor := NewMonitor(client, nil, testBasicMonitorOpts)

		monitor.Close()

		monitor.watchToken()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		callbacks := MonitorCallbacks{OnToken: func(string, bool) {
			panic("test panic")
		}}
		monitor := NewMonitor(client, &callbacks, testBasicMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		err := client.AuthTokenAdd(ctx, token)
		require.NoError(t, err)
		defer func() {
			err = client.AuthTokenRemove(ctx, token)
			require.NoError(t, err)
		}()

		// wait call OnToken and panic
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("tokens", func(t *testing.T) {
		callbacks := MonitorCallbacks{OnToken: func(token string, add bool) {}}
		monitor := NewMonitor(client, &callbacks, testBasicMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		tokens := monitor.Tokens()
		require.Equal(t, []string{client.GetToken()}, tokens)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("nil tokens", func(t *testing.T) {
		var monitor Monitor

		tokens := monitor.Tokens()
		require.Empty(t, tokens)

		testsuite.IsDestroyed(t, &monitor)
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func testAddJob(ctx context.Context, t *testing.T, client *Client) string {
	name := "multi/handler"
	opts := make(map[string]interface{})
	opts["PAYLOAD"] = "windows/meterpreter/reverse_tcp"
	opts["TARGET"] = 0
	opts["LHOST"] = "127.0.0.1"
	opts["LPORT"] = "0"
	result, err := client.ModuleExecute(ctx, "exploit", name, opts)
	require.NoError(t, err)
	return strconv.FormatUint(result.JobID, 10)
}

func TestMonitor_jobMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("active", func(t *testing.T) {
		// add a job before start monitor for first watch
		firstJobID := testAddJob(ctx, t, client)
		defer func() {
			err := client.JobStop(ctx, firstJobID)
			require.NoError(t, err)
		}()

		var (
			sID     string
			sName   string
			sActive bool
		)
		callbacks := MonitorCallbacks{OnJob: func(id, name string, active bool) {
			sID = id
			sName = name
			sActive = active
		}}
		monitor := NewMonitor(client, &callbacks, testBasicMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		jobID := testAddJob(ctx, t, client)
		defer func() {
			err := client.JobStop(ctx, jobID)
			require.NoError(t, err)
		}()

		// wait watch
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)

		// compare result
		require.Equal(t, jobID, sID)
		require.True(t, sActive)
		t.Log(sID, sName)
	})

	t.Run("stop", func(t *testing.T) {
		jobID := testAddJob(ctx, t, client)

		var (
			sID   string
			sName string
		)
		sActive := true

		callbacks := MonitorCallbacks{OnJob: func(id, name string, active bool) {
			sID = id
			sName = name
			sActive = active
		}}
		monitor := NewMonitor(client, &callbacks, testBasicMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// wait watch stopped jobs
		err := client.JobStop(ctx, jobID)
		require.NoError(t, err)

		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)

		// compare result
		require.Equal(t, jobID, sID)
		require.False(t, sActive)
		t.Log(sID, sName)
	})

	t.Run("failed to watch", func(t *testing.T) {
		monitor := NewMonitor(client, nil, testBasicMonitorOpts)
		monitor.Close()

		monitor.watchJob()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		callbacks := MonitorCallbacks{OnJob: func(string, string, bool) {
			panic("test panic")
		}}
		monitor := NewMonitor(client, &callbacks, testBasicMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		jobID := testAddJob(ctx, t, client)
		defer func() {
			err := client.JobStop(ctx, jobID)
			require.NoError(t, err)
		}()

		// wait call OnJob and panic
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("jobs", func(t *testing.T) {
		jobID := testAddJob(ctx, t, client)
		defer func() {
			err := client.JobStop(ctx, jobID)
			require.NoError(t, err)
		}()

		callbacks := MonitorCallbacks{OnJob: func(string, string, bool) {}}
		monitor := NewMonitor(client, &callbacks, testBasicMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		jobs := monitor.Jobs()
		require.Len(t, jobs, 1)
		var cJobID string
		for cJobID = range jobs {
		}
		require.Equal(t, jobID, cJobID)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("nil jobs", func(t *testing.T) {
		var monitor Monitor

		jobs := monitor.Jobs()
		require.Empty(t, jobs)

		testsuite.IsDestroyed(t, &monitor)
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestMonitor_sessionMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("first session", func(t *testing.T) {
		id := testCreateShellSession(t, client, "55500")
		defer func() {
			err := client.SessionStop(ctx, id)
			require.NoError(t, err)
		}()
		callbacks := MonitorCallbacks{
			OnJob:     func(id, name string, active bool) {},
			OnSession: func(id uint64, info *SessionInfo, opened bool) {},
		}
		monitor := NewMonitor(client, &callbacks, testBasicMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("opened", func(t *testing.T) {
		var (
			sID     uint64
			sOpened bool
		)
		callbacks := MonitorCallbacks{
			OnJob: func(string, string, bool) {},
			OnSession: func(id uint64, info *SessionInfo, opened bool) {
				sID = id
				sOpened = opened
			},
		}
		monitor := NewMonitor(client, &callbacks, testBasicMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		id := testCreateShellSession(t, client, "55501")
		defer func() {
			err := client.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		// wait watch
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)

		// compare result
		require.Equal(t, id, sID)
		require.True(t, sOpened)
		t.Log(sID)
	})

	t.Run("closed", func(t *testing.T) {
		id := testCreateShellSession(t, client, "55502")

		var sID uint64
		sOpened := true

		callbacks := MonitorCallbacks{
			OnJob: func(string, string, bool) {},
			OnSession: func(id uint64, info *SessionInfo, opened bool) {
				sID = id
				sOpened = opened
			},
		}
		monitor := NewMonitor(client, &callbacks, testBasicMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// wait watch closed sessions
		err := client.SessionStop(ctx, id)
		require.NoError(t, err)

		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)

		// compare result
		require.Equal(t, id, sID)
		require.False(t, sOpened)
		t.Log(sID)
	})

	t.Run("failed to watch", func(t *testing.T) {
		monitor := NewMonitor(client, nil, testBasicMonitorOpts)
		monitor.Close()

		monitor.watchSession()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		callbacks := MonitorCallbacks{
			OnJob: func(string, string, bool) {},
			OnSession: func(uint64, *SessionInfo, bool) {
				panic("test panic")
			},
		}
		monitor := NewMonitor(client, &callbacks, testBasicMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		id := testCreateShellSession(t, client, "55503")
		defer func() {
			err := client.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		// wait call OnSession and panic
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("sessions", func(t *testing.T) {
		id := testCreateShellSession(t, client, "55504")
		defer func() {
			err := client.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		callbacks := MonitorCallbacks{
			OnJob:     func(string, string, bool) {},
			OnSession: func(uint64, *SessionInfo, bool) {},
		}
		monitor := NewMonitor(client, &callbacks, testBasicMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		sessions := monitor.Sessions()
		require.Len(t, sessions, 1)
		var cSessionID uint64
		for cSessionID = range sessions {
		}
		require.Equal(t, id, cSessionID)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("nil sessions", func(t *testing.T) {
		var monitor Monitor

		sessions := monitor.Sessions()
		require.Empty(t, sessions)

		testsuite.IsDestroyed(t, &monitor)
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestMonitor_hostMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()

	const (
		tempWorkspace    = "temp"
		invalidWorkspace = "foo"
	)

	opts := &DBDelHostOptions{
		Address: testDBHost.Host,
	}

	t.Run("add", func(t *testing.T) {
		// must delete or not new host
		_, _ = client.DBDelHost(ctx, opts)

		// add new workspace for create map
		err := client.DBAddWorkspace(ctx, tempWorkspace)
		require.NoError(t, err)
		defer func() {
			err = client.DBDelWorkspace(ctx, tempWorkspace)
			require.NoError(t, err)
		}()

		var (
			sWorkspace string
			sHost      *DBHost
			sAdd       bool
		)
		callbacks := MonitorCallbacks{
			OnHost: func(workspace string, host *DBHost, add bool) {
				sWorkspace = workspace
				sHost = host
				sAdd = add
			},
		}
		monitor := NewMonitor(client, &callbacks, testDBMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// add host
		err = client.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)
		defer func() {
			_, err = client.DBDelHost(ctx, opts)
			require.NoError(t, err)
		}()

		// wait watch
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)

		// compare result
		require.Equal(t, defaultWorkspace, sWorkspace)
		require.Equal(t, testDBHost.Name, sHost.Name)
		require.True(t, sAdd)
	})

	t.Run("delete", func(t *testing.T) {
		err := client.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)
		defer func() {
			_, _ = client.DBDelHost(ctx, opts)
		}()

		var (
			sWorkspace string
			sHost      *DBHost
		)
		sAdd := true

		callbacks := MonitorCallbacks{
			OnHost: func(workspace string, host *DBHost, add bool) {
				sWorkspace = workspace
				sHost = host
				sAdd = add
			},
		}
		monitor := NewMonitor(client, &callbacks, testDBMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// wait watch delete host
		_, err = client.DBDelHost(ctx, opts)
		require.NoError(t, err)

		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)

		// compare result
		require.Equal(t, defaultWorkspace, sWorkspace)
		require.Equal(t, testDBHost.Name, sHost.Name)
		require.False(t, sAdd)
	})

	t.Run("failed to get workspace", func(t *testing.T) {
		monitor := NewMonitor(client, nil, testDBMonitorOpts)

		monitor.Close()

		monitor.watchHost()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("failed to watch", func(t *testing.T) {
		monitor := NewMonitor(client, nil, testDBMonitorOpts)

		monitor.watchHostWithWorkspace(invalidWorkspace)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		// must delete or not new host
		_, _ = client.DBDelHost(ctx, opts)

		callbacks := MonitorCallbacks{OnHost: func(string, *DBHost, bool) {
			panic("test panic")
		}}
		monitor := NewMonitor(client, &callbacks, testDBMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// add host
		err := client.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)
		defer func() {
			_, err = client.DBDelHost(ctx, opts)
			require.NoError(t, err)
		}()

		// wait call OnHost and panic
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("hosts", func(t *testing.T) {
		_, _ = client.DBDelHost(ctx, opts)
		err := client.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)
		defer func() {
			_, err = client.DBDelHost(ctx, opts)
			require.NoError(t, err)
		}()

		callbacks := MonitorCallbacks{OnHost: func(string, *DBHost, bool) {}}
		monitor := NewMonitor(client, &callbacks, testDBMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		hosts, err := monitor.Hosts(defaultWorkspace)
		require.NoError(t, err)
		require.NotEmpty(t, hosts)

		// invalid workspace name
		hosts, err = monitor.Hosts(invalidWorkspace)
		require.Error(t, err)
		require.Nil(t, hosts)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("nil hosts", func(t *testing.T) {
		var monitor Monitor

		hosts, err := monitor.Hosts(defaultWorkspace)
		require.NoError(t, err)
		require.Empty(t, hosts)

		testsuite.IsDestroyed(t, &monitor)
	})

	err := client.DBDisconnect(ctx)
	require.NoError(t, err)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestMonitor_credentialMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()

	const (
		workspace        = ""
		tempWorkspace    = "temp"
		invalidWorkspace = "foo"
	)

	t.Run("add", func(t *testing.T) {
		// must delete or not new credentials
		_, _ = client.DBDelCreds(ctx, workspace)

		// add new workspace for create map
		err := client.DBAddWorkspace(ctx, tempWorkspace)
		require.NoError(t, err)
		defer func() {
			err = client.DBDelWorkspace(ctx, tempWorkspace)
			require.NoError(t, err)
		}()

		var (
			sWorkspace string
			sCred      *DBCred
			sAdd       bool
		)
		callbacks := MonitorCallbacks{
			OnCredential: func(workspace string, cred *DBCred, add bool) {
				sWorkspace = workspace
				sCred = cred
				sAdd = add
			},
		}
		monitor := NewMonitor(client, &callbacks, testDBMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// add credential
		_, err = client.DBCreateCredential(ctx, testDBCred)
		require.NoError(t, err)
		defer func() {
			_, err = client.DBDelCreds(ctx, workspace)
			require.NoError(t, err)
		}()

		// wait watch
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)

		// compare result
		require.Equal(t, defaultWorkspace, sWorkspace)
		require.Equal(t, testDBCred.Username, sCred.Username)
		require.True(t, sAdd)
	})

	t.Run("delete", func(t *testing.T) {
		// add credential
		_, err := client.DBCreateCredential(ctx, testDBCred)
		require.NoError(t, err)
		defer func() {
			_, err = client.DBDelCreds(ctx, workspace)
			require.NoError(t, err)
		}()

		var (
			sWorkspace string
			sCred      *DBCred
		)
		sAdd := true

		callbacks := MonitorCallbacks{
			OnCredential: func(workspace string, cred *DBCred, add bool) {
				sWorkspace = workspace
				sCred = cred
				sAdd = add
			},
		}
		monitor := NewMonitor(client, &callbacks, testDBMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// wait watch delete credential
		_, err = client.DBDelCreds(ctx, workspace)
		require.NoError(t, err)

		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)

		// compare result
		require.Equal(t, defaultWorkspace, sWorkspace)
		require.Equal(t, testDBCred.Username, sCred.Username)
		require.False(t, sAdd)
	})

	t.Run("failed to get workspace", func(t *testing.T) {
		monitor := NewMonitor(client, nil, testDBMonitorOpts)

		monitor.Close()

		monitor.watchCredential()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("failed to watch", func(t *testing.T) {
		monitor := NewMonitor(client, nil, testDBMonitorOpts)

		monitor.watchCredentialWithWorkspace(invalidWorkspace)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		// must delete or not new credentials
		_, _ = client.DBDelCreds(ctx, workspace)

		callbacks := MonitorCallbacks{OnCredential: func(string, *DBCred, bool) {
			panic("test panic")
		}}
		monitor := NewMonitor(client, &callbacks, testDBMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// add credential
		_, err := client.DBCreateCredential(ctx, testDBCred)
		require.NoError(t, err)
		defer func() {
			_, err = client.DBDelCreds(ctx, workspace)
			require.NoError(t, err)
		}()

		// wait call OnCredential and panic
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("credentials", func(t *testing.T) {
		_, _ = client.DBDelCreds(ctx, workspace)
		_, err := client.DBCreateCredential(ctx, testDBCred)
		require.NoError(t, err)
		defer func() {
			_, err = client.DBDelCreds(ctx, workspace)
			require.NoError(t, err)
		}()

		callbacks := MonitorCallbacks{OnCredential: func(string, *DBCred, bool) {}}
		monitor := NewMonitor(client, &callbacks, testDBMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		creds, err := monitor.Credentials(defaultWorkspace)
		require.NoError(t, err)
		require.NotEmpty(t, creds)

		// invalid workspace name
		creds, err = monitor.Credentials(invalidWorkspace)
		require.Error(t, err)
		require.Nil(t, creds)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("nil credentials", func(t *testing.T) {
		var monitor Monitor

		creds, err := monitor.Credentials(defaultWorkspace)
		require.NoError(t, err)
		require.Empty(t, creds)

		testsuite.IsDestroyed(t, &monitor)
	})

	err := client.DBDisconnect(ctx)
	require.NoError(t, err)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestMonitor_lootMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()

	const (
		tempWorkspace    = "temp"
		invalidWorkspace = "foo"
	)

	t.Run("add", func(t *testing.T) {
		// add new workspace for create map
		err := client.DBAddWorkspace(ctx, tempWorkspace)
		require.NoError(t, err)
		defer func() {
			err = client.DBDelWorkspace(ctx, tempWorkspace)
			require.NoError(t, err)
		}()

		var (
			sWorkspace string
			sLoot      *DBLoot
		)

		callbacks := MonitorCallbacks{
			OnLoot: func(workspace string, loot *DBLoot) {
				sWorkspace = workspace
				sLoot = loot
			},
		}
		monitor := NewMonitor(client, &callbacks, testDBMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// add loot
		err = client.DBReportLoot(ctx, testDBLoot)
		require.NoError(t, err)

		// wait watch
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)

		// compare result
		require.Equal(t, defaultWorkspace, sWorkspace)
		require.Equal(t, testDBLoot.Name, sLoot.Name)
	})

	t.Run("failed to get workspace", func(t *testing.T) {
		monitor := NewMonitor(client, nil, testDBMonitorOpts)

		monitor.Close()

		monitor.watchLoot()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("failed to watch", func(t *testing.T) {
		monitor := NewMonitor(client, nil, testDBMonitorOpts)

		monitor.watchLootWithWorkspace(invalidWorkspace)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		callbacks := MonitorCallbacks{OnLoot: func(string, *DBLoot) {
			panic("test panic")
		}}
		monitor := NewMonitor(client, &callbacks, testDBMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// add loot
		err := client.DBReportLoot(ctx, testDBLoot)
		require.NoError(t, err)

		// wait call OnLoot and panic
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("loots", func(t *testing.T) {
		// add loot
		err := client.DBReportLoot(ctx, testDBLoot)
		require.NoError(t, err)

		callbacks := MonitorCallbacks{OnLoot: func(string, *DBLoot) {}}
		monitor := NewMonitor(client, &callbacks, testDBMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		loots, err := monitor.Loots(defaultWorkspace)
		require.NoError(t, err)
		require.NotEmpty(t, loots)

		// invalid workspace name
		loots, err = monitor.Loots(invalidWorkspace)
		require.Error(t, err)
		require.Nil(t, loots)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("nil loots", func(t *testing.T) {
		var monitor Monitor

		loots, err := monitor.Loots(defaultWorkspace)
		require.NoError(t, err)
		require.Empty(t, loots)

		testsuite.IsDestroyed(t, &monitor)
	})

	err := client.DBDisconnect(ctx)
	require.NoError(t, err)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestMonitor_workspaceCleaner(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()

	const workspace = "temp"

	t.Run("clean", func(t *testing.T) {
		// add temporary workspace
		err := client.DBAddWorkspace(ctx, workspace)
		require.NoError(t, err)

		// add test data
		opts := DBDelHostOptions{
			Address: testDBHost.Host,
		}
		_, _ = client.DBDelHost(ctx, &opts)
		err = client.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)

		_, _ = client.DBDelCreds(ctx, workspace)
		_, err = client.DBCreateCredential(ctx, testDBCred)
		require.NoError(t, err)

		err = client.DBReportLoot(ctx, testDBLoot)
		require.NoError(t, err)

		jobID := testAddJob(ctx, t, client)
		defer func() {
			err = client.JobStop(ctx, jobID)
			require.NoError(t, err)
		}()

		// create monitor
		callbacks := MonitorCallbacks{OnJob: func(string, string, bool) {}}
		monitor := NewMonitor(client, &callbacks, testDBMonitorOpts)
		monitor.Start()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		err = client.DBDelWorkspace(ctx, workspace)
		require.NoError(t, err)

		// wait clean workspace
		time.Sleep(2 * time.Second)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("failed to get workspace", func(t *testing.T) {
		monitor := NewMonitor(client, nil, testDBMonitorOpts)

		monitor.Close()

		monitor.cleanWorkspace()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		m := &Client{}
		patch := func(interface{}, context.Context) ([]*DBWorkspace, error) {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(m, "DBWorkspaces", patch)
		defer pg.Unpatch()

		callbacks := MonitorCallbacks{}
		monitor := NewMonitor(client, &callbacks, testDBMonitorOpts)
		monitor.Start()

		// wait clean workspace
		time.Sleep(2 * time.Second)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	err := client.DBDisconnect(ctx)
	require.NoError(t, err)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestMonitor_log(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		username = "foo"
		password = "bar"
	)

	client, err := NewClient(testAddress, username, password, logger.Test, nil)
	require.NoError(t, err)
	client.token = "TEST"

	monitor := NewMonitor(client, nil, testDBMonitorOpts)
	monitor.Start()

	// log before close
	monitor.logf(logger.Debug, "%s", "foo")
	monitor.log(logger.Debug, "foo")

	monitor.Close()

	// log after close
	monitor.logf(logger.Debug, "%s", "foo")
	monitor.log(logger.Debug, "foo")

	testsuite.IsDestroyed(t, monitor)

	err = client.Close()
	require.Error(t, err)
	client.Kill()

	testsuite.IsDestroyed(t, client)
}

func TestMonitor_updateClientErrorCount(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		username = "foo"
		password = "bar"
	)

	client, err := NewClient(testAddress, username, password, logger.Test, nil)
	require.NoError(t, err)
	client.token = "TEST" // skip AuthLogin

	var errStr string
	callbacks := MonitorCallbacks{OnEvent: func(error string) {
		errStr = error
	}}
	monitor := NewMonitor(client, &callbacks, testDBMonitorOpts)
	monitor.Close()
	monitor.inShutdown = 0

	t.Run("msfrpcd disconnect", func(t *testing.T) {
		// mock error
		monitor.updateClientErrorCount(true)

		// 3 times
		monitor.clErrorCount = 2
		monitor.updateClientErrorCount(true)

		require.Equal(t, "msfrpcd disconnected", errStr)
		require.False(t, monitor.ClientAlive())
	})

	t.Run("msfrpcd reconnected", func(t *testing.T) {
		// mock error
		monitor.clErrorCount = 2
		monitor.updateClientErrorCount(true)

		// ok
		monitor.updateClientErrorCount(false)

		require.Equal(t, "msfrpcd reconnected", errStr)
		require.True(t, monitor.ClientAlive())
	})

	t.Run("msfrpcd reconnect failed", func(t *testing.T) {
		client.token = "TEMP"

		monitor.clErrorCount = 2
		monitor.updateClientErrorCount(true)

		testPatchClientSend(func() {
			monitor.updateClientErrorCount(false)
		})
	})

	testsuite.IsDestroyed(t, monitor)

	err = client.Close()
	require.Error(t, err)
	client.Kill()

	testsuite.IsDestroyed(t, client)
}

func TestMonitor_updateDBErrorCount(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClient(t)

	var errStr string
	callbacks := MonitorCallbacks{OnEvent: func(error string) {
		errStr = error
	}}

	// set invalid port
	port := testDBMonitorOpts.Database.Port
	testDBMonitorOpts.Database.Port = 99999
	defer func() {
		testDBMonitorOpts.Database.Port = port
	}()

	monitor := NewMonitor(client, &callbacks, testDBMonitorOpts)
	monitor.Close()
	monitor.inShutdown = 0

	t.Run("database disconnect", func(t *testing.T) {
		// mock error
		monitor.updateDBErrorCount(true)

		// 3 times
		monitor.dbErrorCount = 2
		monitor.updateDBErrorCount(true)

		require.Equal(t, "database disconnected", errStr)
		require.False(t, monitor.DatabaseAlive())
	})

	t.Run("database reconnected", func(t *testing.T) {
		// mock error
		monitor.dbErrorCount = 2
		monitor.updateDBErrorCount(true)

		// ok
		monitor.updateDBErrorCount(false)

		require.Equal(t, "database reconnected", errStr)
		require.True(t, monitor.DatabaseAlive())
	})

	t.Run("database reconnect failed", func(t *testing.T) {
		monitor.dbErrorCount = 2
		monitor.updateDBErrorCount(true)

		testPatchClientSend(func() {
			monitor.updateDBErrorCount(false)
		})
	})

	testsuite.IsDestroyed(t, monitor)

	err := client.Close()
	require.Error(t, err)
	client.Kill()

	testsuite.IsDestroyed(t, client)
}

func TestMonitor_AutoReconnect(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()

	callbacks := MonitorCallbacks{OnEvent: func(event string) {}}
	monitor := NewMonitor(client, &callbacks, testDBMonitorOpts)
	monitor.Start()

	t.Run("msfrpcd", func(t *testing.T) {
		err := client.AuthLogout(client.GetToken())
		require.NoError(t, err)

		time.Sleep(6 * minWatchInterval)
	})

	t.Run("database", func(t *testing.T) {
		err := client.DBDisconnect(ctx)
		require.NoError(t, err)

		time.Sleep(6 * minWatchInterval)
	})

	monitor.Close()

	testsuite.IsDestroyed(t, monitor)

	err := client.DBDisconnect(ctx)
	require.NoError(t, err)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestMonitorOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/monitor_opts.toml")
	require.NoError(t, err)

	// check unnecessary field
	opts := MonitorOptions{}
	err = toml.Unmarshal(data, &opts)
	require.NoError(t, err)

	// check zero value
	testsuite.CheckOptions(t, opts)

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: 30 * time.Second, actual: opts.Interval},
		{expected: true, actual: opts.EnableDB},
		{expected: "postgresql", actual: opts.Database.Driver},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}
