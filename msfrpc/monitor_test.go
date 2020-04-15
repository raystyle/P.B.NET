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

	t.Run("add", func(t *testing.T) {
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

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
