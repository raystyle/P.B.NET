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

func TestMonitor_tokensMonitor(t *testing.T) {
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

		// check result
		mu.Lock()
		defer mu.Unlock()
		require.Equal(t, token, sToken)
		require.True(t, sAdd)
	})

	t.Run("delete", func(t *testing.T) {
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

		err = msfrpc.AuthTokenAdd(ctx, token)
		require.NoError(t, err)
		defer func() {
			err = msfrpc.AuthTokenRemove(ctx, token)
			require.NoError(t, err)
		}()

		// wait watch for update added token
		time.Sleep(3 * minWatchInterval)

		// wait watch for update delete token
		err = msfrpc.AuthTokenRemove(ctx, token)
		require.NoError(t, err)

		time.Sleep(3 * minWatchInterval)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)

		// check result
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

func TestMonitor_jobsMonitor(t *testing.T) {
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

		// check result
		mu.Lock()
		defer mu.Unlock()
		require.Equal(t, jobID, sID)
		require.True(t, sActive)
		t.Log(sID, sName)
	})

	t.Run("stop", func(t *testing.T) {
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

		// wait watch active jobs
		time.Sleep(3 * minWatchInterval)

		// wait watch stopped jobs
		err = msfrpc.JobStop(ctx, jobID)
		require.NoError(t, err)

		time.Sleep(3 * minWatchInterval)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)

		// check result
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
		callbacks := Callbacks{OnToken: func(token string, add bool) {
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
		for id := range jobs {
			cJobID = id
		}
		require.Equal(t, jobID, cJobID)

		monitor.Close()
		testsuite.IsDestroyed(t, monitor)
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMonitor_sessionsMonitor(t *testing.T) {
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

		// check result
		mu.Lock()
		defer mu.Unlock()
		require.Equal(t, id, sID)
		require.True(t, sOpened)
		t.Log(sID)
	})

	t.Run("closed", func(t *testing.T) {

	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}