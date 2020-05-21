package msfrpc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/patch/monkey"
	"project/internal/patch/msgpack"
	"project/internal/testsuite"
)

const (
	testHost     = "127.0.0.1"
	testPort     = "55553"
	testAddress  = testHost + ":" + testPort
	testUsername = "msf"
	testPassword = "msf"

	testInvalidToken = "invalid token"
)

func TestMain(m *testing.M) {
	exitCode := m.Run()
	// create msfrpc
	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Discard, nil)
	testsuite.CheckErrorInTestMain(err)
	err = msfrpc.AuthLogin()
	testsuite.CheckErrorInTestMain(err)
	// check leaks
	ctx := context.Background()
	for _, check := range []func(context.Context, *MSFRPC) bool{
		testMainCheckSession,
		testMainCheckJob,
		testMainCheckConsole,
		testMainCheckToken,
		testMainCheckThread,
	} {
		if !check(ctx, msfrpc) {
			time.Sleep(time.Minute)
			os.Exit(1)
		}
	}
	msfrpc.Kill()
	// one test main goroutine and two goroutine about
	// pprof server in internal/testsuite.go
	leaks := true
	for i := 0; i < 300; i++ {
		if runtime.NumGoroutine() == 3 {
			leaks = false
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if leaks {
		fmt.Println("[warning] goroutine leaks!")
		time.Sleep(time.Minute)
		os.Exit(1)
	}
	if !testsuite.Destroyed(msfrpc) {
		fmt.Println("[warning] msfrpc is not destroyed!")
		time.Sleep(time.Minute)
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func testMainCheckSession(ctx context.Context, msfrpc *MSFRPC) bool {
	var (
		sessions map[uint64]*SessionInfo
		err      error
	)
	for i := 0; i < 30; i++ {
		sessions, err = msfrpc.SessionList(ctx)
		testsuite.CheckErrorInTestMain(err)
		if len(sessions) == 0 {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println("[warning] msfrpcd session leaks!")
	const format = "id: %d type: %s remote: %s\n"
	for id, session := range sessions {
		fmt.Printf(format, id, session.Type, session.TunnelPeer)
	}
	return false
}

func testMainCheckJob(ctx context.Context, msfrpc *MSFRPC) bool {
	var (
		list map[string]string
		err  error
	)
	for i := 0; i < 30; i++ {
		list, err = msfrpc.JobList(ctx)
		testsuite.CheckErrorInTestMain(err)
		if len(list) == 0 {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println("[warning] msfrpcd job leaks!")
	const format = "id: %s name: %s\n"
	for id, name := range list {
		fmt.Printf(format, id, name)
	}
	return false
}

func testMainCheckConsole(ctx context.Context, msfrpc *MSFRPC) bool {
	var (
		consoles []*ConsoleInfo
		err      error
	)
	for i := 0; i < 30; i++ {
		consoles, err = msfrpc.ConsoleList(ctx)
		testsuite.CheckErrorInTestMain(err)
		if len(consoles) == 0 {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println("[warning] msfrpcd console leaks!")
	const format = "id: %s prompt: %s\n"
	for i := 0; i < len(consoles); i++ {
		fmt.Printf(format, consoles[i].ID, consoles[i].Prompt)
	}
	return false
}

func testMainCheckToken(ctx context.Context, msfrpc *MSFRPC) bool {
	var (
		tokens []string
		err    error
	)
	for i := 0; i < 30; i++ {
		tokens, err = msfrpc.AuthTokenList(ctx)
		testsuite.CheckErrorInTestMain(err)
		// include self token
		if len(tokens) == 1 {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println("[warning] msfrpcd token leaks!")
	for i := 0; i < len(tokens); i++ {
		fmt.Println(tokens[i])
	}
	return false
}

func testMainCheckThread(ctx context.Context, msfrpc *MSFRPC) bool {
	var (
		threads map[uint64]*CoreThreadInfo
		err     error
	)
	for i := 0; i < 30; i++ {
		threads, err = msfrpc.CoreThreadList(ctx)
		testsuite.CheckErrorInTestMain(err)
		// TODO [external] msfrpcd thread leaks
		// if you call SessionMeterpreterRead() or SessionMeterpreterWrite()
		// when you exit meterpreter shell. this thread is always sleep.
		// so deceive ourselves now.
		for id, thread := range threads {
			if thread.Name == "StreamMonitorRemote" ||
				thread.Name == "MeterpreterRunSingle" {
				delete(threads, id)
			}
		}
		// 3 = internal(do noting)
		// 9 = start sessions scheduler(5) and session manager(1)
		l := len(threads)
		if l == 3 || l == 9 {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println("[warning] msfrpcd thread leaks!")
	const format = "id: %d\nname: %s\ncritical: %t\nstatus: %s\nstarted: %s\n\n"
	for i, t := range threads {
		fmt.Printf(format, i, t.Name, t.Critical, t.Status, t.Started)
	}
	return false
}

func TestNewMSFRPC(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("ok", func(t *testing.T) {
		msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
		require.NoError(t, err)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("invalid transport option", func(t *testing.T) {
		opts := Options{}
		opts.Transport.TLSClientConfig.RootCAs = []string{"foo ca"}
		msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, &opts)
		require.Error(t, err)
		require.Nil(t, msfrpc)
	})

	t.Run("disable TLS", func(t *testing.T) {
		opts := Options{DisableTLS: true}
		msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)
		require.NotNil(t, msfrpc)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("custom handler", func(t *testing.T) {
		opts := Options{Handler: "hello"}
		msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)
		require.NotNil(t, msfrpc)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})
}

func TestMSFRPC_HijackLogWriter(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)

	msfrpc.HijackLogWriter()

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_sendWithReplace(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("failed to read from", func(t *testing.T) {
		// patch
		client := new(http.Client)
		patch := func(interface{}, *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       testsuite.NewMockReadCloserWithReadError(),
			}, nil
		}
		pg := monkey.PatchInstanceMethod(client, "Do", patch)
		defer pg.Unpatch()

		err = msfrpc.sendWithReplace(ctx, nil, nil, nil)
		testsuite.IsMockReadCloserError(t, errors.Unwrap(err))
	})

	padding := func() {}

	t.Run("ok", func(t *testing.T) {
		request := AuthTokenListRequest{
			Method: MethodAuthTokenList,
			Token:  msfrpc.GetToken(),
		}
		var result AuthTokenListResult
		err = msfrpc.sendWithReplace(ctx, request, &result, padding)
		require.NoError(t, err)
	})

	t.Run("replace", func(t *testing.T) {
		request := AuthTokenListRequest{
			Method: MethodAuthTokenList,
			Token:  msfrpc.GetToken(),
		}
		var result AuthTokenListResult
		err = msfrpc.sendWithReplace(ctx, request, padding, &result)
		require.NoError(t, err)
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_send(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	ctx := context.Background()

	t.Run("invalid request", func(t *testing.T) {
		msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
		require.NoError(t, err)

		err = msfrpc.send(ctx, func() {}, nil)
		require.Error(t, err)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})

	// start mock server(like msfrpcd)
	const testError = "test error"

	serverMux := http.NewServeMux()
	serverMux.HandleFunc("/200", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = msgpack.NewEncoder(w).Encode([]byte("ok"))
	})
	serverMux.HandleFunc("/500_ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		var msfErr MSFError
		msfErr.ErrorMessage = testError
		msfErr.ErrorCode = 500
		_ = msgpack.NewEncoder(w).Encode(msfErr)
	})
	serverMux.HandleFunc("/500_failed", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("invalid data"))
	})
	serverMux.HandleFunc("/401", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	serverMux.HandleFunc("/403", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	serverMux.HandleFunc("/unknown", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	server := http.Server{
		Addr:    "127.0.0.1:0",
		Handler: serverMux,
	}
	port := testsuite.RunHTTPServer(t, "tcp", &server)
	defer func() { _ = server.Close() }()
	address := "127.0.0.1:" + port

	t.Run("internal server error_ok", func(t *testing.T) {
		opts := Options{
			DisableTLS: true,
			Handler:    "500_ok",
		}
		msfrpc, err := NewMSFRPC(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.EqualError(t, err, testError)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("internal server error_failed", func(t *testing.T) {
		opts := Options{
			DisableTLS: true,
			Handler:    "500_failed",
		}
		msfrpc, err := NewMSFRPC(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.Error(t, err)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("unauthorized", func(t *testing.T) {
		opts := Options{
			DisableTLS: true,
			Handler:    "401",
		}
		msfrpc, err := NewMSFRPC(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.EqualError(t, err, "token is invalid")

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("forbidden", func(t *testing.T) {
		opts := Options{
			DisableTLS: true,
			Handler:    "403",
		}
		msfrpc, err := NewMSFRPC(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.EqualError(t, err, "token is not granted access to the resource")

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("not found", func(t *testing.T) {
		opts := Options{
			DisableTLS: true,
			Handler:    "not_found",
		}
		msfrpc, err := NewMSFRPC(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.EqualError(t, err, "the request was sent to an invalid URL")

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)

	})

	t.Run("other status code", func(t *testing.T) {
		opts := Options{
			DisableTLS: true,
			Handler:    "unknown",
		}
		msfrpc, err := NewMSFRPC(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.EqualError(t, err, "202 Accepted")

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("parallel", func(t *testing.T) {
		opts := Options{
			DisableTLS: true,
			Handler:    "200",
		}
		msfrpc, err := NewMSFRPC(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		f1 := func() {
			testdata := []byte{0x00, 0x01}
			var result []byte
			err := msfrpc.send(ctx, &testdata, &result)
			require.NoError(t, err)
			require.Equal(t, []byte("ok"), result)
		}
		f2 := func() {
			testdata := []byte{0x02, 0x03}
			var result []byte
			err := msfrpc.send(ctx, &testdata, &result)
			require.NoError(t, err)
			require.Equal(t, []byte("ok"), result)
		}
		testsuite.RunParallel(f1, f2)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})
}

func testPatchSend(f func()) {
	patch := func(context.Context, string, string, io.Reader) (*http.Request, error) {
		return nil, monkey.Error
	}
	pg := monkey.Patch(http.NewRequestWithContext, patch)
	defer pg.Unpatch()
	f()
}

func TestMSFRPC_GetConsole(t *testing.T) {
	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)

	t.Run("exist", func(t *testing.T) {
		const id = "0"
		console := &Console{id: id}

		add := msfrpc.trackConsole(console, true)
		require.True(t, add)
		defer func() {
			del := msfrpc.trackConsole(console, false)
			require.True(t, del)
		}()

		c, err := msfrpc.GetConsole(id)
		require.NoError(t, err)
		require.Equal(t, console, c)
	})

	t.Run("doesn't exist", func(t *testing.T) {
		console, err := msfrpc.GetConsole("foo id")
		require.EqualError(t, err, "console \"foo id\" doesn't exist")
		require.Nil(t, console)
	})

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_GetShell(t *testing.T) {
	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)

	t.Run("exist", func(t *testing.T) {
		const id uint64 = 0
		shell := &Shell{id: id}

		add := msfrpc.trackShell(shell, true)
		require.True(t, add)
		defer func() {
			del := msfrpc.trackShell(shell, false)
			require.True(t, del)
		}()

		s, err := msfrpc.GetShell(id)
		require.NoError(t, err)
		require.Equal(t, shell, s)
	})

	t.Run("doesn't exist", func(t *testing.T) {
		shell, err := msfrpc.GetShell(999)
		require.EqualError(t, err, "shell \"999\" doesn't exist")
		require.Nil(t, shell)
	})

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_GetMeterpreter(t *testing.T) {
	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)

	t.Run("exist", func(t *testing.T) {
		const id uint64 = 0
		meterpreter := &Meterpreter{id: id}

		add := msfrpc.trackMeterpreter(meterpreter, true)
		require.True(t, add)
		defer func() {
			del := msfrpc.trackMeterpreter(meterpreter, false)
			require.True(t, del)
		}()

		m, err := msfrpc.GetMeterpreter(id)
		require.NoError(t, err)
		require.Equal(t, meterpreter, m)
	})

	t.Run("doesn't exist", func(t *testing.T) {
		meterpreter, err := msfrpc.GetMeterpreter(999)
		require.EqualError(t, err, "meterpreter \"999\" doesn't exist")
		require.Nil(t, meterpreter)
	})

	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_Close(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("ok", func(t *testing.T) {
		msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
		require.NoError(t, err)

		err = msfrpc.AuthLogin()
		require.NoError(t, err)

		err = msfrpc.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("failed to close", func(t *testing.T) {
		msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
		require.NoError(t, err)

		err = msfrpc.Close()
		require.Error(t, err)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})
}

// TODO finish it
func TestOptions(t *testing.T) {

}
