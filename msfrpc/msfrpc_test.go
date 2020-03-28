package msfrpc

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/patch/msgpack"
	"project/internal/testsuite"
)

const (
	testCommand  = "msfrpcd"
	testHost     = "127.0.0.1"
	testPort     = 55553
	testUsername = "msf"
	testPassword = "msf"

	testInvalidToken    = "invalid token"
	testErrInvalidToken = "Invalid Authentication Token"
)

func TestMain(m *testing.M) {
	// start Metasploit RPC service
	cmd := exec.Command(testCommand, "-a", testHost, "-U", testUsername, "-P", testPassword)
	err := cmd.Start()
	if err != nil {
		panic(err)
	}
	// wait some time for start Metasploit RPC service
	// stdout and stderr can't read any data, so use time.Sleep
	// TODO remove comment
	// time.Sleep(10 * time.Second)
	// stop Metasploit RPC service
	defer func() {
		_ = cmd.Process.Kill()
	}()
	os.Exit(m.Run())
}

func TestNewMSFRPC(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
		require.NoError(t, err)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("invalid transport option", func(t *testing.T) {
		opts := Options{}
		opts.Transport.TLSClientConfig.RootCAs = []string{"foo ca"}
		msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, &opts)
		require.Error(t, err)
		require.Nil(t, msfrpc)
	})

	t.Run("disable TLS", func(t *testing.T) {
		opts := Options{DisableTLS: true}
		msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, &opts)
		require.NoError(t, err)
		require.NotNil(t, msfrpc)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("custom handler", func(t *testing.T) {
		opts := Options{Handler: "hello"}
		msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, &opts)
		require.NoError(t, err)
		require.NotNil(t, msfrpc)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})
}

func TestMSFRPC_sendWithReplace(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("failed to read from", func(t *testing.T) {
		// patch
		client := new(http.Client)
		patchFunc := func(interface{}, *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       testsuite.NewMockReadCloserWithReadError(),
			}, nil
		}
		pg := monkey.PatchInstanceMethod(client, "Do", patchFunc)
		defer pg.Unpatch()

		err = msfrpc.sendWithReplace(ctx, nil, nil, nil)
		require.EqualError(t, testsuite.ErrMockReadCloser, err.Error())
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
	ctx := context.Background()

	t.Run("invalid request", func(t *testing.T) {
		msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
		require.NoError(t, err)

		err = msfrpc.send(ctx, func() {}, nil)
		require.Error(t, err)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})

	// start mock server(like msfrpcd)
	const testError = "test error"

	serverMux := http.NewServeMux()
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
	portNum, err := strconv.Atoi(port)
	require.NoError(t, err)

	t.Run("internal server error_ok", func(t *testing.T) {
		portNum := uint16(portNum)
		opts := Options{
			DisableTLS: true,
			Handler:    "500_ok",
		}
		msfrpc, err := NewMSFRPC(testHost, portNum, testUsername, testPassword, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.EqualError(t, err, testError)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("internal server error_failed", func(t *testing.T) {
		portNum := uint16(portNum)
		opts := Options{
			DisableTLS: true,
			Handler:    "500_failed",
		}
		msfrpc, err := NewMSFRPC(testHost, portNum, testUsername, testPassword, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.Error(t, err)

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("unauthorized", func(t *testing.T) {
		portNum := uint16(portNum)
		opts := Options{
			DisableTLS: true,
			Handler:    "401",
		}
		msfrpc, err := NewMSFRPC(testHost, portNum, testUsername, testPassword, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.EqualError(t, err, "token is invalid")

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("forbidden", func(t *testing.T) {
		portNum := uint16(portNum)
		opts := Options{
			DisableTLS: true,
			Handler:    "403",
		}
		msfrpc, err := NewMSFRPC(testHost, portNum, testUsername, testPassword, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.EqualError(t, err, "token is not granted access to the resource")

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})

	t.Run("not found", func(t *testing.T) {
		portNum := uint16(portNum)
		opts := Options{
			DisableTLS: true,
			Handler:    "not_found",
		}
		msfrpc, err := NewMSFRPC(testHost, portNum, testUsername, testPassword, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.EqualError(t, err, "the request was sent to an invalid URL")

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)

	})

	t.Run("other status code", func(t *testing.T) {
		portNum := uint16(portNum)
		opts := Options{
			DisableTLS: true,
			Handler:    "unknown",
		}
		msfrpc, err := NewMSFRPC(testHost, portNum, testUsername, testPassword, &opts)
		require.NoError(t, err)

		err = msfrpc.send(ctx, nil, nil)
		require.EqualError(t, err, "202 Accepted")

		msfrpc.Kill()
		testsuite.IsDestroyed(t, msfrpc)
	})
}

func testPatchSend(f func()) {
	patch := func(context.Context, string, string, io.Reader) (*http.Request, error) {
		return nil, monkey.ErrMonkey
	}
	pg := monkey.Patch(http.NewRequestWithContext, patch)
	defer pg.Unpatch()
	f()
}

func TestMSFRPC_Login(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)

	t.Run("login success", func(t *testing.T) {
		err = msfrpc.Login()
		require.NoError(t, err)
	})

	t.Run("login failed", func(t *testing.T) {
		msfrpc.password = "foo"
		err = msfrpc.Login()
		require.EqualError(t, err, "Login Failed")

		msfrpc.password = testUsername
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			err = msfrpc.Login()
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_Logout(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)

	t.Run("logout self", func(t *testing.T) {
		err = msfrpc.Login()
		require.NoError(t, err)

		err = msfrpc.Logout(msfrpc.GetToken())
		require.NoError(t, err)
	})

	t.Run("logout invalid token", func(t *testing.T) {
		err = msfrpc.Login()
		require.NoError(t, err)

		err = msfrpc.Logout(testInvalidToken)
		require.EqualError(t, err, testErrInvalidToken)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			err = msfrpc.Logout(msfrpc.GetToken())
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_TokenList(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		token := msfrpc.GetToken()
		list, err := msfrpc.TokenList()
		require.NoError(t, err)
		var exist bool
		for i := 0; i < len(list); i++ {
			t.Log(list[i])
			if token == list[i] {
				exist = true
			}
		}
		require.True(t, exist)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		list, err := msfrpc.TokenList()
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, list)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			list, err := msfrpc.TokenList()
			monkey.IsMonkeyError(t, err)
			require.Nil(t, list)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_TokenGenerate(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		token, err := msfrpc.TokenGenerate()
		require.NoError(t, err)
		t.Log(token)

		tokens, err := msfrpc.TokenList()
		require.NoError(t, err)
		require.Contains(t, tokens, token)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		token, err := msfrpc.TokenGenerate()
		require.EqualError(t, err, testErrInvalidToken)
		require.Equal(t, "", token)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			token, err := msfrpc.TokenGenerate()
			monkey.IsMonkeyError(t, err)
			require.Equal(t, "", token)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_TokenAdd(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	const token = "TEST0123456789012345678901234567"

	t.Run("success", func(t *testing.T) {
		err := msfrpc.TokenAdd(token)
		require.NoError(t, err)

		tokens, err := msfrpc.TokenList()
		require.NoError(t, err)
		require.Contains(t, tokens, token)
	})

	t.Run("add invalid token", func(t *testing.T) {
		err := msfrpc.TokenAdd(testInvalidToken)
		require.NoError(t, err)

		tokens, err := msfrpc.TokenList()
		require.NoError(t, err)
		require.Contains(t, tokens, testInvalidToken)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		// due to the last sub test added testInvalidToken,
		// so must change the token that will be set
		msfrpc.SetToken(testInvalidToken + "foo")
		err := msfrpc.TokenAdd(token)
		require.EqualError(t, err, testErrInvalidToken)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.TokenAdd(token)
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_TokenRemove(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	const token = "TEST0123456789012345678901234567"

	t.Run("success", func(t *testing.T) {
		err := msfrpc.TokenRemove(token)
		require.NoError(t, err)

		tokens, err := msfrpc.TokenList()
		require.NoError(t, err)
		require.NotContains(t, tokens, token)
	})

	t.Run("remove invalid token", func(t *testing.T) {
		err := msfrpc.TokenAdd(testInvalidToken)
		require.NoError(t, err)

		err = msfrpc.TokenRemove(testInvalidToken)
		require.NoError(t, err)

		// doesn't exists
		err = msfrpc.TokenRemove(testInvalidToken)
		require.NoError(t, err)

		tokens, err := msfrpc.TokenList()
		require.NoError(t, err)
		require.NotContains(t, tokens, testInvalidToken)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		err := msfrpc.TokenRemove(token)
		require.EqualError(t, err, testErrInvalidToken)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.TokenRemove(token)
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_Close(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)

	t.Run("ok", func(t *testing.T) {
		err = msfrpc.Login()
		require.NoError(t, err)
		err = msfrpc.Close()
		require.NoError(t, err)
	})

	t.Run("failed", func(t *testing.T) {
		err = msfrpc.Close()
		require.Error(t, err)
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
