package msfrpc

import (
	"context"
	"encoding/hex"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/module/shellcode"
	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func testCreateShellSession(t *testing.T, msfrpc *MSFRPC, port string) {
	testCreateSession(t, msfrpc, "shell", port)
}

func testCreateMeterpreterSession(t *testing.T, msfrpc *MSFRPC, port string) {
	testCreateSession(t, msfrpc, "meterpreter", port)
}

func testCreateSession(t *testing.T, msfrpc *MSFRPC, typ, port string) {
	ctx := context.Background()

	// select payload
	const exploit = "multi/handler"
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
	opts["EXITFUNC"] = "thread"
	opts["TARGET"] = 0
	opts["LHOST"] = "127.0.0.1"
	opts["LPORT"] = port

	// start handler
	result, err := msfrpc.ModuleExecute(ctx, "exploit", exploit, opts)
	require.NoError(t, err)
	var ok bool
	defer func() {
		if ok {
			return
		}
		jobID := strconv.FormatUint(result.JobID, 10)
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
	result, err = msfrpc.ModuleExecute(ctx, "payload", payload, payloadOpts)
	require.NoError(t, err)
	sc := []byte(result.Payload)
	t.Log("raw payload:", hex.EncodeToString(sc))

	// execute shellcode and wait
	go func() { _ = shellcode.Execute("", sc) }()
	time.Sleep(5 * time.Second)

	// check session number
	sessions, err := msfrpc.SessionList(ctx)
	require.NoError(t, err)
	if len(sessions) == 1 {
		ok = true
	}
}

func TestMSFRPC_SessionList(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
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

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		testCreateShellSession(t, msfrpc, "55001")

		sessions, err := msfrpc.SessionList(ctx)
		require.NoError(t, err)
		for id := range sessions {
			err = msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}
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

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		testCreateShellSession(t, msfrpc, "55002")

		sessions, err := msfrpc.SessionList(ctx)
		require.NoError(t, err)
		for id := range sessions {
			result, err := msfrpc.SessionRead(ctx, id, 0)
			require.NoError(t, err)
			t.Log(result.Seq, result.Data)

			err = msfrpc.SessionStop(ctx, id)
			require.NoError(t, err)
		}
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
