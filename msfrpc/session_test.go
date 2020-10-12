package msfrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/module/shellcode"
	"project/internal/patch/monkey"
	"project/internal/system"
	"project/internal/testsuite"
)

func testCreateShellSession(t *testing.T, client *Client, port string) uint64 {
	return testCreateSession(t, client, "shell", port)
}

func testCreateMeterpreterSession(t *testing.T, client *Client, port string) uint64 {
	return testCreateSession(t, client, "meterpreter", port)
}

func testCreateSession(t *testing.T, client *Client, typ, port string) uint64 {
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
	mResult, err := client.ModuleExecute(ctx, "exploit", "multi/handler", opts)
	require.NoError(t, err)
	defer func() {
		jobID := strconv.FormatUint(mResult.JobID, 10)
		err = client.JobStop(ctx, jobID)
		require.NoError(t, err)
	}()

	// generate payload
	payload := opts["PAYLOAD"].(string)
	payloadOpts := NewModuleExecuteOptions()
	payloadOpts.Format = "raw"
	payloadOpts.DataStore["EXITFUNC"] = "thread"
	payloadOpts.DataStore["LHOST"] = "127.0.0.1"
	payloadOpts.DataStore["LPORT"] = port
	pResult, err := client.ModuleExecute(ctx, "payload", payload, payloadOpts)
	require.NoError(t, err)
	sc := []byte(pResult.Payload)
	// execute shellcode and wait some time
	go func() { _ = shellcode.Execute("", sc) }()
	time.Sleep(5 * time.Second)

	// check session number
	sessions, err := client.SessionList(ctx)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	for id := range sessions {
		return id
	}
	return 0
}

func TestClient_SessionList(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		testCreateShellSession(t, client, "55001")

		sessions, err := client.SessionList(ctx)
		require.NoError(t, err)
		for id, session := range sessions {
			const format = "id: %d type: %s remote: %s\n"
			t.Logf(format, id, session.Type, session.TunnelPeer)

			err = client.SessionStop(ctx, id)
			require.NoError(t, err)
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		sessions, err := client.SessionList(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, sessions)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			sessions, err := client.SessionList(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, sessions)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_SessionStop(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id := testCreateShellSession(t, client, "55002")

		err := client.SessionStop(ctx, id)
		require.NoError(t, err)
	})

	t.Run("invalid session id", func(t *testing.T) {
		err := client.SessionStop(ctx, 999)
		require.EqualError(t, err, "unknown session id: 999")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		err := client.SessionStop(ctx, 999)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			err := client.SessionStop(ctx, 999)
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_SessionShellRead(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id := testCreateShellSession(t, client, "55003")
		defer func() {
			err := client.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		result, err := client.SessionShellRead(ctx, id)
		require.NoError(t, err)
		t.Log(result.Seq, result.Data)
	})

	t.Run("invalid session id", func(t *testing.T) {
		result, err := client.SessionShellRead(ctx, 999)
		require.EqualError(t, err, "unknown session id: 999")
		require.Nil(t, result)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		result, err := client.SessionShellRead(ctx, 999)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, result)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			result, err := client.SessionShellRead(ctx, 999)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, result)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_SessionShellWrite(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id := testCreateShellSession(t, client, "55004")
		defer func() {
			err := client.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		result, err := client.SessionShellRead(ctx, id)
		require.NoError(t, err)
		t.Log(result.Seq, result.Data)

		n, err := client.SessionShellWrite(ctx, id, "whoami\n")
		require.NoError(t, err)
		require.Equal(t, uint64(7), n)

		result, err = client.SessionShellRead(ctx, id)
		require.NoError(t, err)
		t.Log(result.Seq, result.Data)
	})

	t.Run("no data", func(t *testing.T) {
		n, err := client.SessionShellWrite(ctx, 0, "")
		require.NoError(t, err)
		require.Zero(t, n)
	})

	const (
		id   = 999
		data = "cmd"
	)

	t.Run("invalid session id", func(t *testing.T) {
		n, err := client.SessionShellWrite(ctx, id, data)
		require.EqualError(t, err, "unknown session id: 999")
		require.Zero(t, n)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		n, err := client.SessionShellWrite(ctx, id, data)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Zero(t, n)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			n, err := client.SessionShellWrite(ctx, id, data)
			monkey.IsMonkeyError(t, err)
			require.Zero(t, n)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

// testCreateShellSessionWithProgram will return file path and session id.
func testCreateShellSessionWithProgram(t *testing.T, client *Client, port string) (string, uint64) {
	ctx := context.Background()

	// select payload
	opts := make(map[string]interface{})
	payloadOpts := NewModuleExecuteOptions()
	switch runtime.GOOS {
	case "windows":
		payloadOpts.Format = "exe"
		switch runtime.GOARCH {
		case "386":
			opts["PAYLOAD"] = "windows/shell/reverse_tcp"
			payloadOpts.Template = TemplateX86WindowsEXE
		case "amd64":
			opts["PAYLOAD"] = "windows/x64/shell/reverse_tcp"
			payloadOpts.Template = TemplateX64WindowsEXE
		default:
			t.Skip("only support 386 and amd64")
		}
	case "linux":
		payloadOpts.Format = "elf"
		switch runtime.GOARCH {
		case "386":
			opts["PAYLOAD"] = "linux/shell/reverse_tcp"
			payloadOpts.Template = TemplateX86LinuxELF
		case "amd64":
			opts["PAYLOAD"] = "linux/x64/shell/reverse_tcp"
			payloadOpts.Template = TemplateX64LinuxELF
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
	mResult, err := client.ModuleExecute(ctx, "exploit", "multi/handler", opts)
	require.NoError(t, err)
	defer func() {
		jobID := strconv.FormatUint(mResult.JobID, 10)
		err = client.JobStop(ctx, jobID)
		require.NoError(t, err)
	}()

	// generate executable file
	payloadOpts.DataStore["EXITFUNC"] = "thread"
	payloadOpts.DataStore["LHOST"] = "127.0.0.1"
	payloadOpts.DataStore["LPORT"] = port

	payload := opts["PAYLOAD"].(string)
	pResult, err := client.ModuleExecute(ctx, "payload", payload, payloadOpts)
	require.NoError(t, err)

	// save
	name := strings.ReplaceAll(t.Name(), "/", "_")
	file := fmt.Sprintf("../temp/test/msfrpc/%s.%s", name, payloadOpts.Format)
	err = system.WriteFile(file, []byte(pResult.Payload))
	require.NoError(t, err)

	// run
	err = exec.Command(file).Start()
	require.NoError(t, err)
	time.Sleep(5 * time.Second)

	// check session number
	sessions, err := client.SessionList(ctx)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	for id := range sessions {
		return file, id
	}
	return file, 0
}

func TestClient_SessionUpgrade(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		file, id := testCreateShellSessionWithProgram(t, client, "55005")
		defer func() {
			// wait program exit
			time.Sleep(time.Second)

			err := os.Remove(file)
			require.NoError(t, err)
		}()

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
		result, err := client.ModuleExecute(ctx, "exploit", "multi/handler", opts)
		require.NoError(t, err)
		defer func() {
			jobID := strconv.FormatUint(result.JobID, 10)
			err = client.JobStop(ctx, jobID)
			require.NoError(t, err)
		}()

		_, err = client.SessionUpgrade(ctx, id, host, port, nil, 0)
		require.NoError(t, err)

		time.Sleep(5 * time.Second)

		// list session
		sessions, err := client.SessionList(ctx)
		require.NoError(t, err)
		for id, session := range sessions {
			const format = "id: %d type: %s remote: %s\n"
			t.Logf(format, id, session.Type, session.TunnelPeer)

			err = client.SessionStop(ctx, id)
			require.NoError(t, err)
		}
	})

	const (
		host = "127.0.0.1"
		port = 55006
		wait = 0
	)

	t.Run("invalid session id", func(t *testing.T) {
		result, err := client.SessionUpgrade(ctx, 999, host, port, nil, wait)
		require.EqualError(t, err, "invalid session id: 999")
		require.Nil(t, result)
	})

	file, id := testCreateShellSessionWithProgram(t, client, "55006")
	defer func() {
		// stop session(need create a new msfrpc client)
		client := testGenerateClientAndLogin(t)

		err := client.SessionStop(ctx, id)
		require.NoError(t, err)

		// wait program exit
		time.Sleep(time.Second)

		err = os.Remove(file)
		require.NoError(t, err)

		err = client.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, client)
	}()

	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			time.Sleep(3 * time.Second)
			cancel()
		}()

		_, err := client.SessionUpgrade(ctx, id, host, port, nil, wait)
		require.Error(t, err)
	})

	t.Run("cancel after write", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			time.Sleep(7 * time.Second)
			cancel()
		}()

		_, err := client.SessionUpgrade(ctx, id, host, port, nil, wait)
		require.Error(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		result, err := client.SessionUpgrade(ctx, id, host, port, nil, wait)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, result)
	})

	t.Run("failed to execute", func(t *testing.T) {
		patch := func(interface{}, context.Context, string, string,
			interface{}) (*ModuleExecuteResult, error) {
			return nil, monkey.Error
		}
		pg := monkey.PatchInstanceMethod(client, "ModuleExecute", patch)
		defer pg.Unpatch()

		result, err := client.SessionUpgrade(ctx, id, host, port, nil, wait)
		monkey.IsMonkeyError(t, err)
		require.Nil(t, result)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			result, err := client.SessionUpgrade(ctx, id, host, port, nil, wait)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, result)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_SessionMeterpreterRead(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id := testCreateMeterpreterSession(t, client, "55010")
		defer func() {
			err := client.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		data, err := client.SessionMeterpreterRead(ctx, id)
		require.NoError(t, err)
		t.Log(data)
	})

	t.Run("invalid session id", func(t *testing.T) {
		data, err := client.SessionMeterpreterRead(ctx, 999)
		require.EqualError(t, err, "unknown session id: 999")
		require.Zero(t, data)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		data, err := client.SessionMeterpreterRead(ctx, 999)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Zero(t, data)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			data, err := client.SessionMeterpreterRead(ctx, 999)
			monkey.IsMonkeyError(t, err)
			require.Zero(t, data)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_SessionMeterpreterWrite(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id := testCreateMeterpreterSession(t, client, "55011")
		defer func() {
			err := client.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		err := client.SessionMeterpreterWrite(ctx, id, "sysinfo")
		require.NoError(t, err)

		time.Sleep(time.Second)

		data, err := client.SessionMeterpreterRead(ctx, id)
		require.NoError(t, err)
		t.Logf("\n%s\n", data)
	})

	const (
		id   = 999
		data = "sysinfo"
	)

	t.Run("no data", func(t *testing.T) {
		err := client.SessionMeterpreterWrite(ctx, id, "")
		require.NoError(t, err)
	})

	t.Run("invalid session id", func(t *testing.T) {
		err := client.SessionMeterpreterWrite(ctx, id, data)
		require.EqualError(t, err, "unknown session id: 999")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		err := client.SessionMeterpreterWrite(ctx, id, data)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			err := client.SessionMeterpreterWrite(ctx, id, data)
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_SessionMeterpreterSessionDetach(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id := testCreateMeterpreterSession(t, client, "55012")
		defer func() {
			err := client.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		err := client.SessionMeterpreterSessionDetach(ctx, id)
		require.NoError(t, err)
	})

	const id = 999

	t.Run("invalid session id", func(t *testing.T) {
		err := client.SessionMeterpreterSessionDetach(ctx, id)
		require.EqualError(t, err, "unknown session id: 999")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		err := client.SessionMeterpreterSessionDetach(ctx, id)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			err := client.SessionMeterpreterSessionDetach(ctx, id)
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_SessionMeterpreterSessionKill(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id := testCreateMeterpreterSession(t, client, "55013")
		defer func() {
			err := client.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		err := client.SessionMeterpreterSessionKill(ctx, id)
		require.NoError(t, err)
	})

	const id = 999

	t.Run("invalid session id", func(t *testing.T) {
		err := client.SessionMeterpreterSessionKill(ctx, id)
		require.EqualError(t, err, "unknown session id: 999")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		err := client.SessionMeterpreterSessionKill(ctx, id)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			err := client.SessionMeterpreterSessionKill(ctx, id)
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_SessionMeterpreterRunSingle(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id := testCreateMeterpreterSession(t, client, "55014")
		defer func() {
			err := client.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		err := client.SessionMeterpreterRunSingle(ctx, id, "sysinfo")
		require.NoError(t, err)

		time.Sleep(time.Second)

		data, err := client.SessionMeterpreterRead(ctx, id)
		require.NoError(t, err)
		t.Logf("\n%s\n", data)
	})

	const (
		id   = 999
		data = "sysinfo"
	)

	t.Run("invalid session id", func(t *testing.T) {
		err := client.SessionMeterpreterRunSingle(ctx, id, data)
		require.EqualError(t, err, "unknown session id: 999")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		err := client.SessionMeterpreterRunSingle(ctx, id, data)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			err := client.SessionMeterpreterRunSingle(ctx, id, data)
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_SessionCompatibleModules(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("shell", func(t *testing.T) {
		id := testCreateShellSession(t, client, "55015")

		modules, err := client.SessionCompatibleModules(ctx, id)
		require.NoError(t, err)
		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}

		err = client.SessionStop(ctx, id)
		require.NoError(t, err)
	})

	t.Run("meterpreter", func(t *testing.T) {
		id := testCreateMeterpreterSession(t, client, "55016")
		defer func() {
			err := client.SessionStop(ctx, id)
			require.NoError(t, err)
		}()

		modules, err := client.SessionCompatibleModules(ctx, id)
		require.NoError(t, err)
		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	const id = 999

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		modules, err := client.SessionCompatibleModules(ctx, id)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, modules)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			modules, err := client.SessionCompatibleModules(ctx, id)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

// if print output, Goland will crash(test), so we write to file.
func testSessionPrintOutput(t *testing.T, buf *bytes.Buffer) {
	if !testsuite.InGoland {
		fmt.Println(buf)
	}
	name := strings.ReplaceAll(t.Name(), "/", "_")
	file := fmt.Sprintf("../temp/test/msfrpc/%s.log", name)
	err := system.WriteFile(file, buf.Bytes())
	require.NoError(t, err)
}

func TestShell(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()
	const interval = 25 * time.Millisecond

	id := testCreateShellSession(t, client, "55300")
	shell := client.NewShell(id, interval)

	buf := new(bytes.Buffer)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(buf, shell)
	}()

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
		_, err := shell.Write([]byte(command))
		require.NoError(t, err)
	}

	time.Sleep(time.Second)

	modules, err := shell.CompatibleModules(ctx)
	require.NoError(t, err)
	for i := 0; i < len(modules); i++ {
		t.Log(modules[i])
	}

	err = shell.Stop()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, shell)

	wg.Wait()

	testSessionPrintOutput(t, buf)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestShell_readLoop(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()
	const interval = 25 * time.Millisecond

	id := testCreateShellSession(t, client, "55301")
	defer func() {
		// stop session(need create a new msfrpc client)
		client := testGenerateClientAndLogin(t)

		err := client.SessionStop(ctx, id)
		require.NoError(t, err)

		err = client.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, client)
	}()

	t.Run("after msfrpc client closed", func(t *testing.T) {
		atomic.StoreInt32(&client.inShutdown, 1)
		defer atomic.StoreInt32(&client.inShutdown, 0)

		shell := client.NewShell(id, interval)

		// wait close self
		time.Sleep(time.Second)

		err := shell.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, shell)
	})

	t.Run("failed to read", func(t *testing.T) {
		shell := client.NewShell(id, interval)

		go func() { _, _ = io.Copy(ioutil.Discard, shell) }()

		time.Sleep(2 * minReadInterval)
		shell.cancel()

		err := shell.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, shell)
	})

	t.Run("panic", func(t *testing.T) {
		_, w := io.Pipe()
		defer func() { _ = w.Close() }()

		patch := func(interface{}) bool {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(w, "Write", patch)
		defer pg.Unpatch()

		shell := client.NewShell(id, interval)

		go func() { _, _ = io.Copy(ioutil.Discard, shell) }()

		time.Sleep(time.Second)

		_, err := shell.Write([]byte("whoami\r\n"))
		require.NoError(t, err)

		time.Sleep(time.Second)

		err = shell.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, shell)
	})

	t.Run("tracker", func(t *testing.T) {
		shell := client.NewShell(id, interval)

		// wait shell tracker
		time.Sleep(time.Second)

		err := client.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, shell)
	})

	client.Kill()

	testsuite.IsDestroyed(t, client)
}

func TestShell_read(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)

	shell := client.NewShell(999, 25*time.Millisecond)

	err := shell.Close()
	require.NoError(t, err)

	ok := shell.read()
	require.False(t, ok)

	testsuite.IsDestroyed(t, shell)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestShell_writeLimiter(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClient(t)

	// force set for prevent net/http call time.Reset()
	client.client.Transport.(*http.Transport).IdleConnTimeout = 0

	err := client.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = client.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const interval = 25 * time.Millisecond

	id := testCreateShellSession(t, client, "55301")
	defer func() {
		// stop session(need create a new msfrpc client)
		client := testGenerateClientAndLogin(t)

		err = client.SessionStop(ctx, id)
		require.NoError(t, err)

		err := client.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, client)
	}()

	t.Run("cancel", func(t *testing.T) {
		shell := client.NewShell(id, interval)

		go func() { _, _ = io.Copy(ioutil.Discard, shell) }()

		time.Sleep(minReadInterval)

		err = shell.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, shell)
	})

	t.Run("panic", func(t *testing.T) {
		timer := time.NewTimer(time.Second)
		defer timer.Stop()

		patch := func(interface{}, time.Duration) bool {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(timer, "Reset", patch)
		defer pg.Unpatch()

		shell := client.NewShell(id, interval)

		time.Sleep(time.Second)

		select {
		case <-shell.token:
		case <-time.After(time.Second):
		}

		// prevent select context
		time.Sleep(time.Second)

		pg.Unpatch()

		err = shell.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, shell)
	})

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestShell_Write(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()
	const interval = 25 * time.Millisecond

	id := testCreateShellSession(t, client, "55301")
	defer func() {
		// stop session(need create a new msfrpc client)
		client := testGenerateClientAndLogin(t)

		err := client.SessionStop(ctx, id)
		require.NoError(t, err)

		err = client.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, client)
	}()

	shell := client.NewShell(id, interval)

	go func() { _, _ = io.Copy(ioutil.Discard, shell) }()

	err := shell.Close()
	require.NoError(t, err)

	_, err = shell.Write([]byte("whoami"))
	require.Equal(t, context.Canceled, err)

	testsuite.IsDestroyed(t, shell)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestShell_Stop(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	const interval = 25 * time.Millisecond

	shell := client.NewShell(999, interval)
	err := shell.Stop()
	require.Error(t, err)
	err = shell.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, shell)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestMeterpreter(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()
	const interval = 25 * time.Millisecond

	id := testCreateMeterpreterSession(t, client, "55400")
	meterpreter := client.NewMeterpreter(id, interval)

	buf := new(bytes.Buffer)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(buf, meterpreter)
	}()

	time.Sleep(time.Second)

	for _, command := range []string{
		"sysinfo",
		"dir",
		"ipconfig",
	} {
		_, err := meterpreter.Write([]byte(command))
		require.NoError(t, err)
	}

	time.Sleep(time.Second)

	modules, err := meterpreter.CompatibleModules(ctx)
	require.NoError(t, err)
	for i := 0; i < len(modules); i++ {
		t.Log(modules[i])
	}

	err = meterpreter.RunSingle(ctx, "dir")
	require.NoError(t, err)
	time.Sleep(time.Second)

	err = meterpreter.Stop()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, meterpreter)

	wg.Wait()

	testSessionPrintOutput(t, buf)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestMeterpreter_readLoop(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()
	const interval = 25 * time.Millisecond

	id := testCreateMeterpreterSession(t, client, "55401")
	defer func() {
		// stop session(need create a new msfrpc client)
		client := testGenerateClientAndLogin(t)

		err := client.SessionStop(ctx, id)
		require.NoError(t, err)

		err = client.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, client)
	}()

	t.Run("after msfrpc client closed", func(t *testing.T) {
		atomic.StoreInt32(&client.inShutdown, 1)
		defer atomic.StoreInt32(&client.inShutdown, 0)

		meterpreter := client.NewMeterpreter(id, interval)

		// wait close self
		time.Sleep(time.Second)

		err := meterpreter.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, meterpreter)
	})

	t.Run("failed to read", func(t *testing.T) {
		meterpreter := client.NewMeterpreter(id, interval)

		go func() { _, _ = io.Copy(ioutil.Discard, meterpreter) }()

		time.Sleep(2 * minReadInterval)
		meterpreter.cancel()

		err := meterpreter.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, meterpreter)
	})

	t.Run("panic", func(t *testing.T) {
		_, w := io.Pipe()
		defer func() { _ = w.Close() }()

		patch := func(interface{}) bool {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(w, "Write", patch)
		defer pg.Unpatch()

		meterpreter := client.NewMeterpreter(id, interval)

		go func() { _, _ = io.Copy(ioutil.Discard, meterpreter) }()

		time.Sleep(time.Second)

		_, err := meterpreter.Write([]byte("whoami"))
		require.NoError(t, err)

		time.Sleep(time.Second)

		err = meterpreter.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, meterpreter)
	})

	t.Run("tracker", func(t *testing.T) {
		meterpreter := client.NewMeterpreter(id, interval)

		// wait meterpreter tracker
		time.Sleep(time.Second)

		err := client.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, meterpreter)
	})

	client.Kill()

	testsuite.IsDestroyed(t, client)
}

func TestMeterpreter_read(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)

	meterpreter := client.NewMeterpreter(999, 25*time.Millisecond)

	err := meterpreter.Close()
	require.NoError(t, err)

	ok := meterpreter.read()
	require.False(t, ok)

	testsuite.IsDestroyed(t, meterpreter)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestMeterpreter_writeLimiter(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClient(t)

	// force set for prevent net/http call time.Reset()
	client.client.Transport.(*http.Transport).IdleConnTimeout = 0

	err := client.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = client.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const interval = 25 * time.Millisecond

	id := testCreateMeterpreterSession(t, client, "55402")
	defer func() {
		// stop session(need create a new msfrpc client)
		client := testGenerateClientAndLogin(t)

		err = client.SessionStop(ctx, id)
		require.NoError(t, err)

		err := client.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, client)
	}()

	t.Run("cancel", func(t *testing.T) {
		meterpreter := client.NewMeterpreter(id, interval)

		go func() { _, _ = io.Copy(ioutil.Discard, meterpreter) }()

		time.Sleep(minReadInterval)

		err = meterpreter.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, meterpreter)
	})

	t.Run("panic", func(t *testing.T) {
		timer := time.NewTimer(time.Second)
		defer timer.Stop()

		patch := func(interface{}, time.Duration) bool {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(timer, "Reset", patch)
		defer pg.Unpatch()

		meterpreter := client.NewMeterpreter(id, interval)

		time.Sleep(time.Second)

		select {
		case <-meterpreter.token:
		case <-time.After(time.Second):
		}

		// prevent select context
		time.Sleep(time.Second)

		pg.Unpatch()

		err = meterpreter.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, meterpreter)
	})

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestMeterpreter_Write(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()
	const interval = 25 * time.Millisecond

	id := testCreateMeterpreterSession(t, client, "55403")
	defer func() {
		// stop session(need create a new msfrpc client)
		client := testGenerateClientAndLogin(t)

		err := client.SessionStop(ctx, id)
		require.NoError(t, err)

		err = client.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, client)
	}()

	meterpreter := client.NewMeterpreter(id, interval)

	go func() { _, _ = io.Copy(ioutil.Discard, meterpreter) }()

	err := meterpreter.Close()
	require.NoError(t, err)

	_, err = meterpreter.Write([]byte("whoami"))
	require.Equal(t, context.Canceled, err)

	testsuite.IsDestroyed(t, meterpreter)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestMeterpreter_Detach(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()
	const interval = 25 * time.Millisecond

	id := testCreateMeterpreterSession(t, client, "55404")
	defer func() {
		// stop session(need create a new msfrpc client)
		client := testGenerateClientAndLogin(t)

		err := client.SessionStop(ctx, id)
		require.NoError(t, err)

		err = client.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, client)
	}()
	time.Sleep(3 * time.Second)

	t.Run("success", func(t *testing.T) {
		meterpreter := client.NewMeterpreter(id, interval)

		buf := new(bytes.Buffer)
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = io.Copy(buf, meterpreter)
		}()

		_, err := meterpreter.Write([]byte("sysinfo"))
		require.NoError(t, err)

		_, err = meterpreter.Write([]byte("shell"))
		require.NoError(t, err)

		// wait shell open
		time.Sleep(3 * time.Second)

		var commands []string
		switch runtime.GOOS {
		case "windows":
			commands = []string{
				"whoami",
				"dir",
				"net user",
				"ipconfig",
			}
		case "linux":
			commands = []string{
				"whoami",
				"ls",
				"ifconfig",
			}
		default:
			t.Skip("only support windows and linux")
		}
		for _, command := range commands {
			_, err = meterpreter.Write([]byte(command))
			require.NoError(t, err)
		}

		time.Sleep(time.Second)

		err = meterpreter.Detach(ctx)
		require.NoError(t, err)

		time.Sleep(time.Second)

		// check is exist
		_, err = meterpreter.Write([]byte("sysinfo"))
		require.NoError(t, err)
		time.Sleep(time.Second)

		err = meterpreter.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, meterpreter)

		wg.Wait()

		testSessionPrintOutput(t, buf)
	})

	t.Run("failed", func(t *testing.T) {
		meterpreter := client.NewMeterpreter(999, interval)

		err := meterpreter.Detach(ctx)
		require.Error(t, err)

		err = meterpreter.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, meterpreter)
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestMeterpreter_Interrupt(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()
	const interval = 25 * time.Millisecond

	id := testCreateMeterpreterSession(t, client, "55405")
	defer func() {
		// stop session(need create a new msfrpc client)
		client := testGenerateClientAndLogin(t)

		err := client.SessionStop(ctx, id)
		require.NoError(t, err)

		err = client.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, client)
	}()

	t.Run("success", func(t *testing.T) {
		meterpreter := client.NewMeterpreter(id, interval)

		buf := new(bytes.Buffer)
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = io.Copy(buf, meterpreter)
		}()

		_, err := meterpreter.Write([]byte("sysinfo"))
		require.NoError(t, err)

		_, err = meterpreter.Write([]byte("shell"))
		require.NoError(t, err)

		// wait shell open
		time.Sleep(3 * time.Second)

		var commands []string
		switch runtime.GOOS {
		case "windows":
			commands = []string{
				"whoami",
				"dir",
				"net user",
				"ipconfig",
			}
		case "linux":
			commands = []string{
				"whoami",
				"ls",
				"ifconfig",
			}
		default:
			t.Skip("only support windows and linux")
		}
		for _, command := range commands {
			_, err = meterpreter.Write([]byte(command))
			require.NoError(t, err)
		}

		time.Sleep(time.Second)

		err = meterpreter.Interrupt(ctx)
		require.NoError(t, err)

		time.Sleep(time.Second)

		// check is exist
		_, err = meterpreter.Write([]byte("sysinfo"))
		require.NoError(t, err)
		time.Sleep(time.Second)

		err = meterpreter.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, meterpreter)

		wg.Wait()

		testSessionPrintOutput(t, buf)
	})

	t.Run("failed", func(t *testing.T) {
		meterpreter := client.NewMeterpreter(999, interval)

		err := meterpreter.Interrupt(ctx)
		require.Error(t, err)

		err = meterpreter.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, meterpreter)
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestMeterpreter_Stop(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	const interval = 25 * time.Millisecond

	meterpreter := client.NewMeterpreter(999, interval)
	err := meterpreter.Stop()
	require.Error(t, err)
	err = meterpreter.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, meterpreter)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}
