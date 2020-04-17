package msfrpc

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func TestMonitor_tokenMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	const (
		interval = 25 * time.Millisecond
		token    = "TEST0123456789012345678901234567"
	)
	ctx := context.Background()

	t.Run("add", func(t *testing.T) {
		var (
			sToken string
			sAdd   bool
			mu     sync.Mutex
		)

		callbacks := Callbacks{OnToken: func(token string, add bool) {
			mu.Lock()
			defer mu.Unlock()
			sToken = token
			sAdd = add
		}}
		monitor := msfrpc.NewMonitor(&callbacks, interval)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		err = msfrpc.AuthTokenAdd(ctx, token)
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
		mu.Lock()
		defer mu.Unlock()
		require.Equal(t, token, sToken)
		require.True(t, sAdd)
	})

	t.Run("delete", func(t *testing.T) {
		err = msfrpc.AuthTokenAdd(ctx, token)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.AuthTokenRemove(ctx, token)
			require.NoError(t, err)
		}()

		var (
			sToken string
			mu     sync.Mutex
		)
		sAdd := true

		callbacks := Callbacks{OnToken: func(token string, add bool) {
			mu.Lock()
			defer mu.Unlock()
			sToken = token
			sAdd = add
		}}
		monitor := msfrpc.NewMonitor(&callbacks, interval)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// wait watch for delete token
		err = msfrpc.AuthTokenRemove(ctx, token)
		require.NoError(t, err)

		time.Sleep(3 * minWatchInterval)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)

		// compare result
		mu.Lock()
		defer mu.Unlock()
		require.Equal(t, token, sToken)
		require.False(t, sAdd)
	})

	t.Run("failed to watch", func(t *testing.T) {
		callbacks := Callbacks{OnToken: func(token string, add bool) {}}
		monitor := msfrpc.NewMonitor(&callbacks, interval)

		err := msfrpc.AuthLogout(msfrpc.GetToken())
		require.NoError(t, err)
		defer func() {
			err = msfrpc.AuthLogin()
			require.NoError(t, err)
		}()

		time.Sleep(3 * minWatchInterval)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		callbacks := Callbacks{OnToken: func(token string, add bool) {
			panic("test panic")
		}}
		monitor := msfrpc.NewMonitor(&callbacks, interval)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		err = msfrpc.AuthTokenAdd(ctx, token)
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
		monitor := msfrpc.NewMonitor(&callbacks, interval)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		tokens := monitor.Tokens()
		require.Equal(t, []string{msfrpc.GetToken()}, tokens)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMonitor_jobMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	const interval = 25 * time.Millisecond
	ctx := context.Background()

	addJob := func() string {
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

	t.Run("active", func(t *testing.T) {
		// add a job before start monitor for first watch
		firstJobID := addJob()
		defer func() {
			err = msfrpc.JobStop(ctx, firstJobID)
			require.NoError(t, err)
		}()

		var (
			sID     string
			sName   string
			sActive bool
			mu      sync.Mutex
		)
		callbacks := Callbacks{OnJob: func(id, name string, active bool) {
			mu.Lock()
			defer mu.Unlock()
			sID = id
			sName = name
			sActive = active
		}}
		monitor := msfrpc.NewMonitor(&callbacks, interval)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		jobID := addJob()
		defer func() {
			err = msfrpc.JobStop(ctx, jobID)
			require.NoError(t, err)
		}()

		// wait watch
		time.Sleep(3 * minWatchInterval)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)

		// compare result
		mu.Lock()
		defer mu.Unlock()
		require.Equal(t, jobID, sID)
		require.True(t, sActive)
		t.Log(sID, sName)
	})

	t.Run("stop", func(t *testing.T) {
		jobID := addJob()

		var (
			sID   string
			sName string
			mu    sync.Mutex
		)
		sActive := true

		callbacks := Callbacks{OnJob: func(id, name string, active bool) {
			mu.Lock()
			defer mu.Unlock()
			sID = id
			sName = name
			sActive = active
		}}
		monitor := msfrpc.NewMonitor(&callbacks, interval)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// wait watch stopped jobs
		err = msfrpc.JobStop(ctx, jobID)
		require.NoError(t, err)

		time.Sleep(3 * minWatchInterval)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)

		// compare result
		mu.Lock()
		defer mu.Unlock()
		require.Equal(t, jobID, sID)
		require.False(t, sActive)
		t.Log(sID, sName)
	})

	t.Run("failed to watch", func(t *testing.T) {
		callbacks := Callbacks{OnJob: func(id, name string, active bool) {}}
		monitor := msfrpc.NewMonitor(&callbacks, interval)

		err := msfrpc.AuthLogout(msfrpc.GetToken())
		require.NoError(t, err)
		defer func() {
			err = msfrpc.AuthLogin()
			require.NoError(t, err)
		}()

		time.Sleep(3 * minWatchInterval)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		callbacks := Callbacks{OnJob: func(id, name string, active bool) {
			panic("test panic")
		}}
		monitor := msfrpc.NewMonitor(&callbacks, interval)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		jobID := addJob()
		defer func() {
			err = msfrpc.JobStop(ctx, jobID)
			require.NoError(t, err)
		}()

		// wait call OnJob and panic
		time.Sleep(3 * minWatchInterval)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("jobs", func(t *testing.T) {
		jobID := addJob()
		defer func() {
			err = msfrpc.JobStop(ctx, jobID)
			require.NoError(t, err)
		}()

		callbacks := Callbacks{OnJob: func(id, name string, active bool) {}}
		monitor := msfrpc.NewMonitor(&callbacks, interval)

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

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMonitor_sessionMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	const interval = 25 * time.Millisecond
	ctx := context.Background()

	t.Run("first session", func(t *testing.T) {
		id := testCreateShellSession(t, msfrpc, "55500")
		defer func() {
			err = msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}()
		callbacks := Callbacks{
			OnJob:     func(id, name string, active bool) {},
			OnSession: func(id uint64, info *SessionInfo, opened bool) {},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("opened", func(t *testing.T) {
		var (
			sID     uint64
			sOpened bool
			mu      sync.Mutex
		)
		callbacks := Callbacks{
			OnJob: func(id, name string, active bool) {},
			OnSession: func(id uint64, info *SessionInfo, opened bool) {
				mu.Lock()
				defer mu.Unlock()
				sID = id
				sOpened = opened
			},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		id := testCreateShellSession(t, msfrpc, "55501")
		defer func() {
			err = msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		// wait watch
		time.Sleep(3 * minWatchInterval)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)

		// compare result
		mu.Lock()
		defer mu.Unlock()
		require.Equal(t, id, sID)
		require.True(t, sOpened)
		t.Log(sID)
	})

	t.Run("closed", func(t *testing.T) {
		id := testCreateShellSession(t, msfrpc, "55502")

		var (
			sID uint64
			mu  sync.Mutex
		)
		sOpened := true

		callbacks := Callbacks{
			OnJob: func(id, name string, active bool) {},
			OnSession: func(id uint64, info *SessionInfo, opened bool) {
				mu.Lock()
				defer mu.Unlock()
				sID = id
				sOpened = opened
			},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// wait watch closed sessions
		err = msfrpc.SessionStop(ctx, id)
		require.NoError(t, err)

		time.Sleep(3 * minWatchInterval)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)

		// compare result
		mu.Lock()
		defer mu.Unlock()
		require.Equal(t, id, sID)
		require.False(t, sOpened)
		t.Log(sID)
	})

	t.Run("failed to watch", func(t *testing.T) {
		callbacks := Callbacks{
			OnJob:     func(id, name string, active bool) {},
			OnSession: func(id uint64, info *SessionInfo, opened bool) {},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval)

		err := msfrpc.AuthLogout(msfrpc.GetToken())
		require.NoError(t, err)
		defer func() {
			err = msfrpc.AuthLogin()
			require.NoError(t, err)
		}()

		time.Sleep(3 * minWatchInterval)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		callbacks := Callbacks{
			OnJob: func(id, name string, active bool) {},
			OnSession: func(id uint64, info *SessionInfo, opened bool) {
				panic("test panic")
			},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval)

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		id := testCreateShellSession(t, msfrpc, "55503")
		defer func() {
			err = msfrpc.SessionStop(ctx, id)
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
			err = msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		callbacks := Callbacks{
			OnJob:     func(id, name string, active bool) {},
			OnSession: func(id uint64, info *SessionInfo, opened bool) {},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval)

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

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMonitor_hostMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	const (
		interval      = 25 * time.Millisecond
		workspace     = ""
		tempWorkspace = "temp"
	)
	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	t.Run("add", func(t *testing.T) {
		// must delete or not new host
		_, _ = msfrpc.DBDelHost(ctx, workspace, testDBHost.Host)

		// add new workspace for watchHostWithWorkspace() about create map
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
			mu         sync.Mutex
		)
		callbacks := Callbacks{
			OnHost: func(workspace string, host *DBHost, add bool) {
				mu.Lock()
				defer mu.Unlock()
				sWorkspace = workspace
				sHost = host
				sAdd = add
			},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval)
		monitor.StartDatabaseMonitors()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// add host
		err = msfrpc.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)
		defer func() {
			_, err = msfrpc.DBDelHost(ctx, workspace, testDBHost.Host)
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
			_, _ = msfrpc.DBDelHost(ctx, workspace, testDBHost.Host)
		}()

		var (
			sWorkspace string
			sHost      *DBHost
			mu         sync.Mutex
		)
		sAdd := true

		callbacks := Callbacks{
			OnHost: func(workspace string, host *DBHost, add bool) {
				mu.Lock()
				defer mu.Unlock()
				sWorkspace = workspace
				sHost = host
				sAdd = add
			},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval)
		monitor.StartDatabaseMonitors()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// wait watch delete host
		_, err = msfrpc.DBDelHost(ctx, workspace, testDBHost.Host)
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
		callbacks := Callbacks{OnHost: func(workspace string, host *DBHost, add bool) {}}
		monitor := msfrpc.NewMonitor(&callbacks, interval)
		monitor.StartDatabaseMonitors()

		err := msfrpc.AuthLogout(msfrpc.GetToken())
		require.NoError(t, err)
		defer func() {
			err = msfrpc.AuthLogin()
			require.NoError(t, err)
		}()

		time.Sleep(3 * minWatchInterval)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("failed to watch", func(t *testing.T) {
		callbacks := Callbacks{OnHost: func(workspace string, host *DBHost, add bool) {}}
		monitor := msfrpc.NewMonitor(&callbacks, interval)
		monitor.StartDatabaseMonitors()

		monitor.watchHostWithWorkspace("foo")

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("panic", func(t *testing.T) {
		// must delete or not new host
		_, _ = msfrpc.DBDelHost(ctx, workspace, testDBHost.Host)

		callbacks := Callbacks{OnHost: func(workspace string, host *DBHost, add bool) {
			panic("test panic")
		}}
		monitor := msfrpc.NewMonitor(&callbacks, interval)
		monitor.StartDatabaseMonitors()

		// wait first watch
		time.Sleep(3 * minWatchInterval)

		// add host
		err := msfrpc.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)
		defer func() {
			_, err = msfrpc.DBDelHost(ctx, workspace, testDBHost.Host)
			require.NoError(t, err)
		}()

		// wait call OnHost and panic
		time.Sleep(3 * minWatchInterval)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)
	})

	t.Run("hosts", func(t *testing.T) {
		_, _ = msfrpc.DBDelHost(ctx, workspace, testDBHost.Host)
		err := msfrpc.DBReportHost(ctx, testDBHost)
		require.NoError(t, err)
		defer func() {
			_, err = msfrpc.DBDelHost(ctx, workspace, testDBHost.Host)
			require.NoError(t, err)
		}()

		callbacks := Callbacks{OnHost: func(workspace string, host *DBHost, add bool) {}}
		monitor := msfrpc.NewMonitor(&callbacks, interval)
		monitor.StartDatabaseMonitors()

		time.Sleep(3 * minWatchInterval)

		hosts, err := monitor.Hosts(defaultWorkspace)
		require.NoError(t, err)
		require.NotEmpty(t, hosts)

		// invalid workspace name
		hosts, err = monitor.Hosts("foo")
		require.Error(t, err)
		require.Nil(t, hosts)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMonitor_credentialMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	const (
		interval      = 25 * time.Millisecond
		workspace     = ""
		tempWorkspace = "temp"
	)
	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	t.Run("add", func(t *testing.T) {
		// must delete or not new credentials
		_, _ = msfrpc.DBDelCreds(ctx, workspace)

		// add new workspace for watchHostWithWorkspace() about create map
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
			mu         sync.Mutex
		)
		callbacks := Callbacks{
			OnCredential: func(workspace string, cred *DBCred, add bool) {
				mu.Lock()
				defer mu.Unlock()
				sWorkspace = workspace
				sCred = cred
				sAdd = add
			},
		}
		monitor := msfrpc.NewMonitor(&callbacks, interval)
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
		require.Equal(t, testDBCred["username"], sCred.Username)
		require.True(t, sAdd)
	})

	err = msfrpc.DBDisconnect(ctx)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
