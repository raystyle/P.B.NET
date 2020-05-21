package msfrpc

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestMonitor_tokenMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndLogin(t)

	const (
		interval = 25 * time.Millisecond
		token    = "TEST0123456789012345678901234567"
	)
	ctx := context.Background()

	t.Run("add", func(t *testing.T) {
		var (
			sToken string
			sAdd   bool
		)

		callbacks := Callbacks{OnToken: func(token string, add bool) {
			sToken = token
			sAdd = add
		}}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		err := msfrpc.AuthTokenAdd(ctx, token)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.AuthTokenRemove(ctx, token)
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
		err := msfrpc.AuthTokenAdd(ctx, token)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.AuthTokenRemove(ctx, token)
			require.NoError(t, err)
		}()

		var sToken string
		sAdd := true

		callbacks := Callbacks{OnToken: func(token string, add bool) {
			sToken = token
			sAdd = add
		}}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// wait watch for delete token
		err = msfrpc.AuthTokenRemove(ctx, token)
		require.NoError(t, err)

		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)

		// compare result
		require.Equal(t, token, sToken)
		require.False(t, sAdd)
	})

	t.Run("failed to watch", func(t *testing.T) {
		monitor := msfrpc.NewMonitor(new(Callbacks), interval, testDBOptions)
		monitor.Close()

		monitor.watchToken()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		callbacks := Callbacks{OnToken: func(string, bool) {
			panic("test panic")
		}}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		err := msfrpc.AuthTokenAdd(ctx, token)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.AuthTokenRemove(ctx, token)
			require.NoError(t, err)
		}()

		// wait call OnToken and panic
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("tokens", func(t *testing.T) {
		callbacks := Callbacks{OnToken: func(token string, add bool) {}}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		tokens := monitor.Tokens()
		require.Equal(t, []string{msfrpc.GetToken()}, tokens)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	err := msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}

func testAddJob(ctx context.Context, t *testing.T, msfrpc *MSFRPC) string {
	name := "multi/handler"
	opts := make(map[string]interface{})
	opts["PAYLOAD"] = "windows/meterpreter/reverse_tcp"
	opts["TARGET"] = 0
	opts["LHOST"] = "127.0.0.1"
	opts["LPORT"] = "0"
	result, err := msfrpc.ModuleExecute(ctx, "exploit", name, opts)
	require.NoError(t, err)
	return strconv.FormatUint(result.JobID, 10)
}

func TestMonitor_jobMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndLogin(t)
	ctx := context.Background()
	const interval = 25 * time.Millisecond

	t.Run("active", func(t *testing.T) {
		// add a job before start monitor for first watch
		firstJobID := testAddJob(ctx, t, msfrpc)
		defer func() {
			err := msfrpc.JobStop(ctx, firstJobID)
			require.NoError(t, err)
		}()

		var (
			sID     string
			sName   string
			sActive bool
		)
		callbacks := Callbacks{OnJob: func(id, name string, active bool) {
			sID = id
			sName = name
			sActive = active
		}}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		jobID := testAddJob(ctx, t, msfrpc)
		defer func() {
			err := msfrpc.JobStop(ctx, jobID)
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
		jobID := testAddJob(ctx, t, msfrpc)

		var (
			sID   string
			sName string
		)
		sActive := true

		callbacks := Callbacks{OnJob: func(id, name string, active bool) {
			sID = id
			sName = name
			sActive = active
		}}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// wait watch stopped jobs
		err := msfrpc.JobStop(ctx, jobID)
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
		monitor := msfrpc.NewMonitor(new(Callbacks), interval, testDBOptions)
		monitor.Close()

		monitor.watchJob()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		callbacks := Callbacks{OnJob: func(string, string, bool) {
			panic("test panic")
		}}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		jobID := testAddJob(ctx, t, msfrpc)
		defer func() {
			err := msfrpc.JobStop(ctx, jobID)
			require.NoError(t, err)
		}()

		// wait call OnJob and panic
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("jobs", func(t *testing.T) {
		jobID := testAddJob(ctx, t, msfrpc)
		defer func() {
			err := msfrpc.JobStop(ctx, jobID)
			require.NoError(t, err)
		}()

		callbacks := Callbacks{OnJob: func(string, string, bool) {}}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)

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

	err := msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMonitor_sessionMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndLogin(t)
	ctx := context.Background()
	const interval = 25 * time.Millisecond

	t.Run("first session", func(t *testing.T) {
		id := testCreateShellSession(t, msfrpc, "55500")
		defer func() {
			err := msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}()
		callbacks := Callbacks{
			OnJob:     func(id, name string, active bool) {},
			OnSession: func(id uint64, info *SessionInfo, opened bool) {},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)

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
		callbacks := Callbacks{
			OnJob: func(string, string, bool) {},
			OnSession: func(id uint64, info *SessionInfo, opened bool) {
				sID = id
				sOpened = opened
			},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		id := testCreateShellSession(t, msfrpc, "55501")
		defer func() {
			err := msfrpc.SessionStop(ctx, id)
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
		id := testCreateShellSession(t, msfrpc, "55502")

		var sID uint64
		sOpened := true

		callbacks := Callbacks{
			OnJob: func(string, string, bool) {},
			OnSession: func(id uint64, info *SessionInfo, opened bool) {
				sID = id
				sOpened = opened
			},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// wait watch closed sessions
		err := msfrpc.SessionStop(ctx, id)
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
		monitor := msfrpc.NewMonitor(new(Callbacks), interval, testDBOptions)
		monitor.Close()

		monitor.watchSession()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		callbacks := Callbacks{
			OnJob: func(string, string, bool) {},
			OnSession: func(uint64, *SessionInfo, bool) {
				panic("test panic")
			},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		id := testCreateShellSession(t, msfrpc, "55503")
		defer func() {
			err := msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		// wait call OnSession and panic
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("sessions", func(t *testing.T) {
		id := testCreateShellSession(t, msfrpc, "55504")
		defer func() {
			err := msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		callbacks := Callbacks{
			OnJob:     func(string, string, bool) {},
			OnSession: func(uint64, *SessionInfo, bool) {},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)

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

	err := msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMonitor_hostMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndConnectDB(t)
	ctx := context.Background()

	const (
		interval         = 25 * time.Millisecond
		tempWorkspace    = "temp"
		invalidWorkspace = "foo"
	)

	opts := &DBDelHostOptions{
		Address: testDBHost.Host,
	}

	t.Run("add", func(t *testing.T) {
		// must delete or not new host
		_, _ = msfrpc.DBDelHost(ctx, opts)

		// add new workspace for create map
		err := msfrpc.DBAddWorkspace(ctx, tempWorkspace)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBDelWorkspace(ctx, tempWorkspace)
			require.NoError(t, err)
		}()

		var (
			sWorkspace string
			sHost      *DBHost
			sAdd       bool
		)
		callbacks := Callbacks{
			OnHost: func(workspace string, host *DBHost, add bool) {
				sWorkspace = workspace
				sHost = host
				sAdd = add
			},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)
		monitor.StartDatabaseMonitors()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// add host
		err = msfrpc.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)
		defer func() {
			_, err = msfrpc.DBDelHost(ctx, opts)
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
		err := msfrpc.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)
		defer func() {
			_, _ = msfrpc.DBDelHost(ctx, opts)
		}()

		var (
			sWorkspace string
			sHost      *DBHost
		)
		sAdd := true

		callbacks := Callbacks{
			OnHost: func(workspace string, host *DBHost, add bool) {
				sWorkspace = workspace
				sHost = host
				sAdd = add
			},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)
		monitor.StartDatabaseMonitors()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// wait watch delete host
		_, err = msfrpc.DBDelHost(ctx, opts)
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
		monitor := msfrpc.NewMonitor(new(Callbacks), interval, testDBOptions)
		monitor.StartDatabaseMonitors()
		monitor.Close()

		monitor.watchHost()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("failed to watch", func(t *testing.T) {
		monitor := msfrpc.NewMonitor(new(Callbacks), interval, testDBOptions)
		monitor.StartDatabaseMonitors()

		monitor.watchHostWithWorkspace(invalidWorkspace)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		// must delete or not new host
		_, _ = msfrpc.DBDelHost(ctx, opts)

		callbacks := Callbacks{OnHost: func(string, *DBHost, bool) {
			panic("test panic")
		}}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)
		monitor.StartDatabaseMonitors()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// add host
		err := msfrpc.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)
		defer func() {
			_, err = msfrpc.DBDelHost(ctx, opts)
			require.NoError(t, err)
		}()

		// wait call OnHost and panic
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("hosts", func(t *testing.T) {
		_, _ = msfrpc.DBDelHost(ctx, opts)
		err := msfrpc.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)
		defer func() {
			_, err = msfrpc.DBDelHost(ctx, opts)
			require.NoError(t, err)
		}()

		callbacks := Callbacks{OnHost: func(string, *DBHost, bool) {}}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)
		monitor.StartDatabaseMonitors()

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

	err := msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	err = msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMonitor_credentialMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndConnectDB(t)
	ctx := context.Background()

	const (
		interval         = 25 * time.Millisecond
		workspace        = ""
		tempWorkspace    = "temp"
		invalidWorkspace = "foo"
	)

	t.Run("add", func(t *testing.T) {
		// must delete or not new credentials
		_, _ = msfrpc.DBDelCreds(ctx, workspace)

		// add new workspace for create map
		err := msfrpc.DBAddWorkspace(ctx, tempWorkspace)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBDelWorkspace(ctx, tempWorkspace)
			require.NoError(t, err)
		}()

		var (
			sWorkspace string
			sCred      *DBCred
			sAdd       bool
		)
		callbacks := Callbacks{
			OnCredential: func(workspace string, cred *DBCred, add bool) {
				sWorkspace = workspace
				sCred = cred
				sAdd = add
			},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)
		monitor.StartDatabaseMonitors()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// add credential
		_, err = msfrpc.DBCreateCredential(ctx, testDBCred)
		require.NoError(t, err)
		defer func() {
			_, err = msfrpc.DBDelCreds(ctx, workspace)
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
		_, err := msfrpc.DBCreateCredential(ctx, testDBCred)
		require.NoError(t, err)
		defer func() {
			_, err = msfrpc.DBDelCreds(ctx, workspace)
			require.NoError(t, err)
		}()

		var (
			sWorkspace string
			sCred      *DBCred
		)
		sAdd := true

		callbacks := Callbacks{
			OnCredential: func(workspace string, cred *DBCred, add bool) {
				sWorkspace = workspace
				sCred = cred
				sAdd = add
			},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)
		monitor.StartDatabaseMonitors()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// wait watch delete credential
		_, err = msfrpc.DBDelCreds(ctx, workspace)
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
		monitor := msfrpc.NewMonitor(new(Callbacks), interval, testDBOptions)
		monitor.StartDatabaseMonitors()
		monitor.Close()

		monitor.watchCredential()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("failed to watch", func(t *testing.T) {
		monitor := msfrpc.NewMonitor(new(Callbacks), interval, testDBOptions)
		monitor.StartDatabaseMonitors()

		monitor.watchCredentialWithWorkspace(invalidWorkspace)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		// must delete or not new credentials
		_, _ = msfrpc.DBDelCreds(ctx, workspace)

		callbacks := Callbacks{OnCredential: func(string, *DBCred, bool) {
			panic("test panic")
		}}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)
		monitor.StartDatabaseMonitors()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// add credential
		_, err := msfrpc.DBCreateCredential(ctx, testDBCred)
		require.NoError(t, err)
		defer func() {
			_, err = msfrpc.DBDelCreds(ctx, workspace)
			require.NoError(t, err)
		}()

		// wait call OnCredential and panic
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("credentials", func(t *testing.T) {
		_, _ = msfrpc.DBDelCreds(ctx, workspace)
		_, err := msfrpc.DBCreateCredential(ctx, testDBCred)
		require.NoError(t, err)
		defer func() {
			_, err = msfrpc.DBDelCreds(ctx, workspace)
			require.NoError(t, err)
		}()

		callbacks := Callbacks{OnCredential: func(string, *DBCred, bool) {}}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)
		monitor.StartDatabaseMonitors()

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

	err := msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	err = msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMonitor_lootMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndConnectDB(t)
	ctx := context.Background()

	const (
		interval         = 25 * time.Millisecond
		tempWorkspace    = "temp"
		invalidWorkspace = "foo"
	)

	t.Run("add", func(t *testing.T) {
		// add new workspace for create map
		err := msfrpc.DBAddWorkspace(ctx, tempWorkspace)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.DBDelWorkspace(ctx, tempWorkspace)
			require.NoError(t, err)
		}()

		var (
			sWorkspace string
			sLoot      *DBLoot
		)

		callbacks := Callbacks{
			OnLoot: func(workspace string, loot *DBLoot) {
				sWorkspace = workspace
				sLoot = loot
			},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)
		monitor.StartDatabaseMonitors()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// add loot
		err = msfrpc.DBReportLoot(ctx, testDBLoot)
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
		monitor := msfrpc.NewMonitor(new(Callbacks), interval, testDBOptions)
		monitor.StartDatabaseMonitors()
		monitor.Close()

		monitor.watchLoot()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("failed to watch", func(t *testing.T) {
		monitor := msfrpc.NewMonitor(new(Callbacks), interval, testDBOptions)
		monitor.StartDatabaseMonitors()

		monitor.watchLootWithWorkspace(invalidWorkspace)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		callbacks := Callbacks{OnLoot: func(string, *DBLoot) {
			panic("test panic")
		}}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)
		monitor.StartDatabaseMonitors()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// add loot
		err := msfrpc.DBReportLoot(ctx, testDBLoot)
		require.NoError(t, err)

		// wait call OnLoot and panic
		time.Sleep(3 * minWatchInterval)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("loots", func(t *testing.T) {
		// add loot
		err := msfrpc.DBReportLoot(ctx, testDBLoot)
		require.NoError(t, err)

		callbacks := Callbacks{OnLoot: func(string, *DBLoot) {}}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)
		monitor.StartDatabaseMonitors()

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

	err := msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	err = msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMonitor_workspaceCleaner(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndConnectDB(t)
	ctx := context.Background()

	const (
		interval  = 25 * time.Millisecond
		workspace = "temp"
	)

	t.Run("clean", func(t *testing.T) {
		// add temporary workspace
		err := msfrpc.DBAddWorkspace(ctx, workspace)
		require.NoError(t, err)

		// add test data
		opts := DBDelHostOptions{
			Address: testDBHost.Host,
		}
		_, _ = msfrpc.DBDelHost(ctx, &opts)
		err = msfrpc.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)

		_, _ = msfrpc.DBDelCreds(ctx, workspace)
		_, err = msfrpc.DBCreateCredential(ctx, testDBCred)
		require.NoError(t, err)

		err = msfrpc.DBReportLoot(ctx, testDBLoot)
		require.NoError(t, err)

		jobID := testAddJob(ctx, t, msfrpc)
		defer func() {
			err = msfrpc.JobStop(ctx, jobID)
			require.NoError(t, err)
		}()

		// create monitor
		callbacks := Callbacks{OnJob: func(string, string, bool) {}}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)
		monitor.StartDatabaseMonitors()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		err = msfrpc.DBDelWorkspace(ctx, workspace)
		require.NoError(t, err)

		// wait clean workspace
		time.Sleep(2 * time.Second)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("failed to get workspace", func(t *testing.T) {
		monitor := msfrpc.NewMonitor(new(Callbacks), interval, testDBOptions)
		monitor.StartDatabaseMonitors()
		monitor.Close()

		monitor.cleanWorkspace()

		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		m := &MSFRPC{}
		patch := func(interface{}, context.Context) ([]*DBWorkspace, error) {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(m, "DBWorkspaces", patch)
		defer pg.Unpatch()

		callbacks := Callbacks{}
		monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)
		monitor.StartDatabaseMonitors()

		// wait clean workspace
		time.Sleep(2 * time.Second)

		monitor.Close()

		testsuite.IsDestroyed(t, monitor)
	})

	err := msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	err = msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMonitor_updateMSFErrorCount(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		username = "foo"
		password = "bar"
		interval = 25 * time.Millisecond
	)

	msfrpc, err := NewMSFRPC(testAddress, username, password, logger.Test, nil)
	require.NoError(t, err)
	msfrpc.token = "TEST"

	var errStr string
	callbacks := Callbacks{OnEvent: func(error string) {
		errStr = error
	}}
	monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)
	monitor.Close()
	monitor.context = context.Background()

	t.Run("msfrpcd disconnect", func(t *testing.T) {
		// mock error
		monitor.updateMSFErrorCount(true)

		// 3 times
		monitor.msfErrorCount = 2
		monitor.updateMSFErrorCount(true)

		require.Equal(t, "msfrpcd disconnected", errStr)
		require.False(t, monitor.MSFRPCDAlive())
	})

	t.Run("msfrpcd reconnected", func(t *testing.T) {
		// mock error
		monitor.updateMSFErrorCount(true)

		// ok
		monitor.updateMSFErrorCount(false)

		require.Equal(t, "msfrpcd reconnected", errStr)
		require.True(t, monitor.MSFRPCDAlive())
	})

	testsuite.IsDestroyed(t, monitor)

	err = msfrpc.Close()
	require.Error(t, err)
	msfrpc.Kill()

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMonitor_updateDBErrorCount(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPC(t)

	const interval = 25 * time.Millisecond

	var errStr string
	callbacks := Callbacks{OnEvent: func(error string) {
		errStr = error
	}}
	dbOpts := *testDBOptions
	dbOpts.Port = 99999
	monitor := msfrpc.NewMonitor(&callbacks, interval, &dbOpts)
	monitor.Close()
	monitor.context = context.Background()

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
		monitor.updateDBErrorCount(true)

		// ok
		monitor.updateDBErrorCount(false)

		require.Equal(t, "database reconnected", errStr)
		require.True(t, monitor.DatabaseAlive())
	})

	testsuite.IsDestroyed(t, monitor)

	err := msfrpc.Close()
	require.Error(t, err)
	msfrpc.Kill()

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMonitor_AutoReconnect(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateMSFRPCAndConnectDB(t)
	ctx := context.Background()
	const interval = 25 * time.Millisecond

	callbacks := Callbacks{OnEvent: func(event string) {}}
	monitor := msfrpc.NewMonitor(&callbacks, interval, testDBOptions)
	monitor.StartDatabaseMonitors()

	t.Run("msfrpcd", func(t *testing.T) {
		err := msfrpc.AuthLogout(msfrpc.GetToken())
		require.NoError(t, err)

		time.Sleep(3 * minWatchInterval)
	})

	t.Run("database", func(t *testing.T) {
		err := msfrpc.DBDisconnect(ctx)
		require.NoError(t, err)

		time.Sleep(3 * minWatchInterval)
	})

	monitor.Close()

	testsuite.IsDestroyed(t, monitor)

	err := msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	err = msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}
