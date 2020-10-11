package msfrpc

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/patch/monkey"
	"project/internal/patch/msgpack"
	"project/internal/patch/toml"
	"project/internal/testsuite"
)

func TestNewMSFRPC(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("invalid transport option", func(t *testing.T) {
		opts := ClientOptions{}
		opts.Transport.TLSClientConfig.RootCAs = []string{"foo ca"}

		msfrpc, err := NewClient(testAddress, testUsername, testPassword, logger.Test, &opts)
		require.Error(t, err)
		require.Nil(t, msfrpc)
	})

	t.Run("not login", func(t *testing.T) {
		msfrpc := testGenerateClient(t)

		err := msfrpc.Close()
		require.Error(t, err)
		msfrpc.Kill()

		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("disable TLS", func(t *testing.T) {
		opts := ClientOptions{DisableTLS: true}

		msfrpc, err := NewClient(testAddress, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)
		require.NotNil(t, msfrpc)

		err = msfrpc.Close()
		require.Error(t, err)
		msfrpc.Kill()

		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("custom handler", func(t *testing.T) {
		opts := ClientOptions{Handler: "hello"}

		msfrpc, err := NewClient(testAddress, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)
		require.NotNil(t, msfrpc)

		err = msfrpc.Close()
		require.Error(t, err)
		msfrpc.Kill()

		testsuite.IsDestroyed(t, msfrpc)
	})
}

func TestClient_HijackLogWriter(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateClient(t)

	msfrpc.HijackLogWriter()

	err := msfrpc.Close()
	require.Error(t, err)
	msfrpc.Kill()

	testsuite.IsDestroyed(t, msfrpc)
}

func TestClient_log(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateClient(t)

	err := msfrpc.Close()
	require.Error(t, err)
	msfrpc.Kill()

	msfrpc.logf(logger.Debug, "%s", "foo")
	msfrpc.log(logger.Debug, "foo")

	testsuite.IsDestroyed(t, msfrpc)
}

func TestClient_sendWithReplace(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc := testGenerateClientAndLogin(t)
	ctx := context.Background()

	padding := func() {}

	t.Run("ok", func(t *testing.T) {
		request := AuthTokenListRequest{
			Method: MethodAuthTokenList,
			Token:  msfrpc.GetToken(),
		}
		var result AuthTokenListResult

		err := msfrpc.sendWithReplace(ctx, request, &result, padding)
		require.NoError(t, err)
	})

	t.Run("replace", func(t *testing.T) {
		request := AuthTokenListRequest{
			Method: MethodAuthTokenList,
			Token:  msfrpc.GetToken(),
		}
		var result AuthTokenListResult

		err := msfrpc.sendWithReplace(ctx, request, padding, &result)
		require.NoError(t, err)
	})

	t.Run("failed to read from-200", func(t *testing.T) {
		client := new(http.Client)
		patch := func(interface{}, *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       testsuite.NewMockConnWithReadError(),
			}, nil
		}
		pg := monkey.PatchInstanceMethod(client, "Do", patch)
		defer pg.Unpatch()

		err := msfrpc.sendWithReplace(ctx, nil, nil, nil)
		testsuite.IsMockConnReadError(t, errors.Unwrap(err))
	})

	t.Run("failed to read from-500", func(t *testing.T) {
		client := new(http.Client)
		patch := func(interface{}, *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       testsuite.NewMockConnWithReadError(),
			}, nil
		}
		pg := monkey.PatchInstanceMethod(client, "Do", patch)
		defer pg.Unpatch()

		err := msfrpc.sendWithReplace(ctx, nil, nil, nil)
		testsuite.IsMockConnReadError(t, errors.Unwrap(err))
	})

	err := msfrpc.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, msfrpc)
}

func TestClient_send(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	ctx := context.Background()

	t.Run("invalid request", func(t *testing.T) {
		msfrpc := testGenerateClient(t)

		err := msfrpc.send(ctx, func() {}, nil)
		require.Error(t, err)

		err = msfrpc.Close()
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
	serverMux.HandleFunc("/unexpected", func(w http.ResponseWriter, r *http.Request) {
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
		opts := ClientOptions{
			DisableTLS: true,
			Handler:    "500_ok",
		}
		msfrpc, err := NewClient(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.EqualError(t, err, testError)

		err = msfrpc.Close()
		require.Error(t, err)
		msfrpc.Kill()

		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("internal server error_failed", func(t *testing.T) {
		opts := ClientOptions{
			DisableTLS: true,
			Handler:    "500_failed",
		}
		msfrpc, err := NewClient(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.Error(t, err)

		err = msfrpc.Close()
		require.Error(t, err)
		msfrpc.Kill()

		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("unauthorized", func(t *testing.T) {
		opts := ClientOptions{
			DisableTLS: true,
			Handler:    "401",
		}
		msfrpc, err := NewClient(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.EqualError(t, err, "invalid token")

		err = msfrpc.Close()
		require.Error(t, err)
		msfrpc.Kill()

		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("forbidden", func(t *testing.T) {
		opts := ClientOptions{
			DisableTLS: true,
			Handler:    "403",
		}
		msfrpc, err := NewClient(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.EqualError(t, err, "this token is not granted access to the resource")

		err = msfrpc.Close()
		require.Error(t, err)
		msfrpc.Kill()

		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("not found", func(t *testing.T) {
		opts := ClientOptions{
			DisableTLS: true,
			Handler:    "not_found",
		}
		msfrpc, err := NewClient(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.EqualError(t, err, "the request was sent to an invalid URL")

		err = msfrpc.Close()
		require.Error(t, err)
		msfrpc.Kill()

		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("unexpected http status code", func(t *testing.T) {
		opts := ClientOptions{
			DisableTLS: true,
			Handler:    "unexpected",
		}
		msfrpc, err := NewClient(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.EqualError(t, err, "unexpected http status code: 202")

		err = msfrpc.Close()
		require.Error(t, err)
		msfrpc.Kill()

		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("parallel", func(t *testing.T) {
		opts := ClientOptions{
			DisableTLS: true,
			Handler:    "200",
		}
		msfrpc, err := NewClient(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		send1 := func() {
			testdata := []byte{0x00, 0x01}
			var result []byte
			err := msfrpc.send(ctx, &testdata, &result)
			require.NoError(t, err)
			require.Equal(t, []byte("ok"), result)
		}
		send2 := func() {
			testdata := []byte{0x02, 0x03}
			var result []byte
			err := msfrpc.send(ctx, &testdata, &result)
			require.NoError(t, err)
			require.Equal(t, []byte("ok"), result)
		}
		testsuite.RunParallel(10, nil, nil, send1, send2)

		err = msfrpc.Close()
		require.Error(t, err)
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

func TestClient_GetConsole(t *testing.T) {
	msfrpc := testGenerateClient(t)

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

func TestClient_GetShell(t *testing.T) {
	msfrpc := testGenerateClient(t)

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

func TestClient_GetMeterpreter(t *testing.T) {
	msfrpc := testGenerateClient(t)

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

func TestClient_Close(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("ok", func(t *testing.T) {
		msfrpc := testGenerateClientAndLogin(t)

		err := msfrpc.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("failed to close", func(t *testing.T) {
		msfrpc := testGenerateClient(t)

		err := msfrpc.Close()
		require.Error(t, err)

		msfrpc.Kill()

		testsuite.IsDestroyed(t, msfrpc)
	})
}

func TestOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/options.toml")
	require.NoError(t, err)

	// check unnecessary field
	opts := ClientOptions{}
	err = toml.Unmarshal(data, &opts)
	require.NoError(t, err)

	// check zero value
	testsuite.CheckOptions(t, opts)

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: true, actual: opts.DisableTLS},
		{expected: true, actual: opts.TLSVerify},
		{expected: "custom", actual: opts.Handler},
		{expected: 30 * time.Second, actual: opts.Timeout},
		{expected: "test_token", actual: opts.Token},
		{expected: 2, actual: opts.Transport.MaxIdleConns},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}
