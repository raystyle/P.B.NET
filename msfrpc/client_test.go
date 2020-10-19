package msfrpc

import (
	"context"
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

func TestNewClient(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("invalid transport option", func(t *testing.T) {
		opts := ClientOptions{}
		opts.Transport.TLSClientConfig.RootCAs = []string{"foo ca"}

		client, err := NewClient(testAddress, testUsername, testPassword, logger.Test, &opts)
		require.Error(t, err)
		require.Nil(t, client)
	})

	t.Run("not login", func(t *testing.T) {
		client := testGenerateClient(t)

		err := client.Close()
		require.Error(t, err)
		client.Kill()

		testsuite.IsDestroyed(t, client)
	})

	t.Run("disable TLS", func(t *testing.T) {
		opts := ClientOptions{DisableTLS: true}

		client, err := NewClient(testAddress, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)
		require.NotNil(t, client)

		err = client.Close()
		require.Error(t, err)
		client.Kill()

		testsuite.IsDestroyed(t, client)
	})

	t.Run("custom handler", func(t *testing.T) {
		opts := ClientOptions{Handler: "hello"}

		client, err := NewClient(testAddress, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)
		require.NotNil(t, client)

		err = client.Close()
		require.Error(t, err)
		client.Kill()

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_HijackLogWriter(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClient(t)

	client.HijackLogWriter()

	err := client.Close()
	require.Error(t, err)
	client.Kill()

	testsuite.IsDestroyed(t, client)
}

func TestClient_log(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClient(t)

	err := client.Close()
	require.Error(t, err)
	client.Kill()

	client.logf(logger.Debug, "%s", "foo")
	client.log(logger.Debug, "foo")

	testsuite.IsDestroyed(t, client)
}

func TestClient_sendWithReplace(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	padding := func() {}

	t.Run("ok", func(t *testing.T) {
		request := AuthTokenListRequest{
			Method: MethodAuthTokenList,
			Token:  client.GetToken(),
		}
		var result AuthTokenListResult

		err := client.sendWithReplace(ctx, request, &result, padding)
		require.NoError(t, err)
	})

	t.Run("replace", func(t *testing.T) {
		request := AuthTokenListRequest{
			Method: MethodAuthTokenList,
			Token:  client.GetToken(),
		}
		var result AuthTokenListResult

		err := client.sendWithReplace(ctx, request, padding, &result)
		require.NoError(t, err)
	})

	t.Run("failed to read from-200", func(t *testing.T) {
		httpClient := new(http.Client)
		patch := func(interface{}, *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       testsuite.NewMockConnWithReadError(),
			}, nil
		}
		pg := monkey.PatchInstanceMethod(httpClient, "Do", patch)
		defer pg.Unpatch()

		err := client.sendWithReplace(ctx, nil, nil, nil)
		testsuite.IsMockConnReadError(t, errors.Unwrap(err))
	})

	t.Run("failed to read from-500", func(t *testing.T) {
		httpClient := new(http.Client)
		patch := func(interface{}, *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       testsuite.NewMockConnWithReadError(),
			}, nil
		}
		pg := monkey.PatchInstanceMethod(httpClient, "Do", patch)
		defer pg.Unpatch()

		err := client.sendWithReplace(ctx, nil, nil, nil)
		testsuite.IsMockConnReadError(t, errors.Unwrap(err))
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_send(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	ctx := context.Background()

	t.Run("invalid request", func(t *testing.T) {
		client := testGenerateClient(t)

		err := client.send(ctx, func() {}, nil)
		require.Error(t, err)

		err = client.Close()
		require.Error(t, err)
		client.Kill()

		testsuite.IsDestroyed(t, client)
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
		client, err := NewClient(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = client.send(ctx, nil, nil)
		require.EqualError(t, err, testError)

		err = client.Close()
		require.Error(t, err)
		client.Kill()

		testsuite.IsDestroyed(t, client)
	})

	t.Run("internal server error_failed", func(t *testing.T) {
		opts := ClientOptions{
			DisableTLS: true,
			Handler:    "500_failed",
		}
		client, err := NewClient(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = client.send(ctx, nil, nil)
		require.Error(t, err)

		err = client.Close()
		require.Error(t, err)
		client.Kill()

		testsuite.IsDestroyed(t, client)
	})

	t.Run("unauthorized", func(t *testing.T) {
		opts := ClientOptions{
			DisableTLS: true,
			Handler:    "401",
		}
		client, err := NewClient(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = client.send(ctx, nil, nil)
		require.EqualError(t, err, "invalid token")

		err = client.Close()
		require.Error(t, err)
		client.Kill()

		testsuite.IsDestroyed(t, client)
	})

	t.Run("forbidden", func(t *testing.T) {
		opts := ClientOptions{
			DisableTLS: true,
			Handler:    "403",
		}
		client, err := NewClient(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = client.send(ctx, nil, nil)
		require.EqualError(t, err, "this token is not granted access to the resource")

		err = client.Close()
		require.Error(t, err)
		client.Kill()

		testsuite.IsDestroyed(t, client)
	})

	t.Run("not found", func(t *testing.T) {
		opts := ClientOptions{
			DisableTLS: true,
			Handler:    "not_found",
		}
		client, err := NewClient(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = client.send(ctx, nil, nil)
		require.EqualError(t, err, "the request was sent to an invalid URL")

		err = client.Close()
		require.Error(t, err)
		client.Kill()

		testsuite.IsDestroyed(t, client)
	})

	t.Run("unexpected http status code", func(t *testing.T) {
		opts := ClientOptions{
			DisableTLS: true,
			Handler:    "unexpected",
		}
		client, err := NewClient(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		err = client.send(ctx, nil, nil)
		require.EqualError(t, err, "unexpected http status code: 202")

		err = client.Close()
		require.Error(t, err)
		client.Kill()

		testsuite.IsDestroyed(t, client)
	})

	t.Run("parallel", func(t *testing.T) {
		opts := ClientOptions{
			DisableTLS: true,
			Handler:    "200",
		}
		client, err := NewClient(address, testUsername, testPassword, logger.Test, &opts)
		require.NoError(t, err)

		send1 := func() {
			testdata := []byte{0x00, 0x01}
			var result []byte
			err := client.send(ctx, &testdata, &result)
			require.NoError(t, err)
			require.Equal(t, []byte("ok"), result)
		}
		send2 := func() {
			testdata := []byte{0x02, 0x03}
			var result []byte
			err := client.send(ctx, &testdata, &result)
			require.NoError(t, err)
			require.Equal(t, []byte("ok"), result)
		}
		testsuite.RunParallel(10, nil, nil, send1, send2)

		err = client.Close()
		require.Error(t, err)
		client.Kill()

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_GetConsole(t *testing.T) {
	client := testGenerateClient(t)

	t.Run("exist", func(t *testing.T) {
		const id = "0"
		console := &Console{id: id}

		add := client.trackConsole(console, true)
		require.True(t, add)
		defer func() {
			del := client.trackConsole(console, false)
			require.True(t, del)
		}()

		c, err := client.GetConsole(id)
		require.NoError(t, err)
		require.Equal(t, console, c)
	})

	t.Run("not exist", func(t *testing.T) {
		console, err := client.GetConsole("999")
		require.EqualError(t, err, "console 999 is not exist")
		require.Nil(t, console)
	})

	testsuite.IsDestroyed(t, client)
}

func TestClient_GetShell(t *testing.T) {
	client := testGenerateClient(t)

	t.Run("exist", func(t *testing.T) {
		const id uint64 = 0
		shell := &Shell{id: id}

		add := client.trackShell(shell, true)
		require.True(t, add)
		defer func() {
			del := client.trackShell(shell, false)
			require.True(t, del)
		}()

		s, err := client.GetShell(id)
		require.NoError(t, err)
		require.Equal(t, shell, s)
	})

	t.Run("not exist", func(t *testing.T) {
		shell, err := client.GetShell(999)
		require.EqualError(t, err, "shell session 999 is not exist")
		require.Nil(t, shell)
	})

	testsuite.IsDestroyed(t, client)
}

func TestClient_GetMeterpreter(t *testing.T) {
	client := testGenerateClient(t)

	t.Run("exist", func(t *testing.T) {
		const id uint64 = 0
		meterpreter := &Meterpreter{id: id}

		add := client.trackMeterpreter(meterpreter, true)
		require.True(t, add)
		defer func() {
			del := client.trackMeterpreter(meterpreter, false)
			require.True(t, del)
		}()

		m, err := client.GetMeterpreter(id)
		require.NoError(t, err)
		require.Equal(t, meterpreter, m)
	})

	t.Run("not exist", func(t *testing.T) {
		meterpreter, err := client.GetMeterpreter(999)
		require.EqualError(t, err, "meterpreter session 999 is not exist")
		require.Nil(t, meterpreter)
	})

	testsuite.IsDestroyed(t, client)
}

func TestClient_Close(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("ok", func(t *testing.T) {
		client := testGenerateClientAndLogin(t)

		err := client.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("failed to close", func(t *testing.T) {
		client := testGenerateClient(t)

		err := client.Close()
		require.Error(t, err)

		client.Kill()

		testsuite.IsDestroyed(t, client)
	})
}

func TestClientOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/client_opts.toml")
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
